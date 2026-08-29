package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"

	"github.com/google/uuid"
)

var (
	ErrSubmissionQueueFull = errors.New("submission queue is full")
	ErrWorkerStopped       = errors.New("order worker is stopped")
)

type orderWorkerStore interface {
	InsertOrder(context.Context, domain.Order) error
	UpdateOrders(context.Context, []domain.Order) error
	PersistSubmission(context.Context, domain.Order, []domain.Order, []domain.Trade, []domain.OrderEvent) error
}

type orderPublisher interface {
	Publish(context.Context, WsTopic, []byte)
}

type OrderWorker struct {
	ctx             context.Context
	store           orderWorkerStore
	engine          *matching.Engine
	symbol          domain.Symbol
	wsServer        orderPublisher
	queue           chan queueCommand
	submissionMutex sync.Mutex
	nextSequence    int64

	mutex   sync.RWMutex
	stopped bool
	done    chan struct{}

	lowerPriceLimit int64
	upperPriceLimit int64
}

type queueCommand struct {
	order *domain.Order
	ready chan bool
}

func NewOrderWorker(
	store orderWorkerStore,
	engine *matching.Engine,
	symbol domain.Symbol,
	wsServer orderPublisher,
	lastSequence int64,
	queueCapacity int,
) (*OrderWorker, error) {
	if queueCapacity <= 0 {
		return nil, errors.New("queue capacity must be > 0")
	}

	lowerPriceLimit, upperPriceLimit := engine.PriceLimits()

	worker := &OrderWorker{
		ctx:             context.Background(),
		store:           store,
		engine:          engine,
		symbol:          symbol,
		wsServer:        wsServer,
		queue:           make(chan queueCommand, queueCapacity),
		nextSequence:    lastSequence + 1,
		done:            make(chan struct{}),
		lowerPriceLimit: lowerPriceLimit,
		upperPriceLimit: upperPriceLimit,
	}

	go worker.run()

	return worker, nil
}

func (worker *OrderWorker) Submit(ctx context.Context, request SubmitOrderRequest) error {
	if err := request.Validate(worker); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	if !strings.EqualFold(request.Symbol, worker.symbol.Code) {
		return fmt.Errorf("%w: symbol does not match worker", ErrInvalidOrder)
	}

	request.Symbol = worker.symbol.Code

	worker.submissionMutex.Lock()
	defer worker.submissionMutex.Unlock()

	if worker.stopped {
		return ErrWorkerStopped
	}

	order, err := CreateOrderFromSubmitOrderRequest(worker, request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	order.Sequence = worker.nextSequence

	err = worker.store.InsertOrder(ctx, order)
	if err != nil {
		log.Printf("Insert order failed: %v", err)
		return err
	}

	worker.nextSequence++

	command := queueCommand{
		order: &order,
		ready: make(chan bool, 1),
	}

	var returnErr error = nil
	select {
	case worker.queue <- command:
		order.Status = domain.StatusPending

	default:
		order.Status = domain.StatusRejected
		returnErr = ErrSubmissionQueueFull
	}

	err = worker.store.UpdateOrders(ctx, []domain.Order{order})
	if err != nil {
		log.Printf("Update order failed: %v", err)
		command.ready <- false
		return err
	}
	command.ready <- true

	return returnErr
}

func (worker *OrderWorker) Stop() {
	worker.mutex.Lock()
	worker.submissionMutex.Lock()
	defer worker.submissionMutex.Unlock()

	if !worker.stopped {
		worker.stopped = true
		close(worker.queue)
	}

	worker.mutex.Unlock()

	<-worker.done
}

func (worker *OrderWorker) run() {
	defer close(worker.done)

	for command := range worker.queue {
		ready := <-command.ready
		order := command.order
		if !ready {
			log.Printf("Order with sequence %d is not ready, update have most likely failed, skipping", order.Sequence)
			continue
		}
		worker.mutex.RLock()

		_, plan, err := worker.process(worker.ctx, *order)
		if err != nil {
			log.Printf(
				"worker failed to process order id=%s sequence=%d: %v",
				order.ID,
				order.Sequence,
				err,
			)

			order.Status = domain.StatusFailed
			err = worker.store.UpdateOrders(worker.ctx, []domain.Order{*order})
			if err != nil {
				log.Printf("[Worker Error] Update order failed: %v", err)
			}
		} else {
			//only fanout last trade price after trade
			if len(plan.Trades) > 0 {
				ltp := worker.engine.LastTradePrice()
				WsFanoutCurrentPrice(ltp, worker)
				WsFanoutTrades(plan, worker)
			}

			WsFanoutMarketDepth(worker)
		}

		worker.mutex.RUnlock()
	}
}

func (worker *OrderWorker) process(ctx context.Context, order domain.Order) ([]domain.OrderEvent, matching.MatchPlan, error) {
	if err := ctx.Err(); err != nil {
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	plan, err := worker.engine.Prepare(order)
	if err != nil {
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	events, err := buildSubmissionEvents(plan)
	if err != nil {
		err = fmt.Errorf("build submission events: %w", err)
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	err = worker.store.PersistSubmission(
		ctx,
		plan.IncomingOrder,
		plan.RestingOrderUpdates,
		plan.Trades,
		events,
	)
	if err != nil {
		err = fmt.Errorf("persist submission: %w", err)
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	if err := worker.engine.Apply(plan); err != nil {
		err = fmt.Errorf("apply persisted match plan: %w", err)
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	return events, plan, nil
}

func CreateOrderFromSubmitOrderRequest(worker *OrderWorker, request SubmitOrderRequest) (domain.Order, error) {
	return domain.NewOrder(
		uuid.NewString(),
		request.ParticipantId,
		worker.symbol,
		worker.engine.SessionDate(),
		request.Side,
		request.Price,
		request.Quantity,
		time.Now().UTC(),
	)
}

func WsFanoutCurrentPrice(ltp int64, worker *OrderWorker) {
	message := LastTradePriceMessage{
		Price: ltp,
	}
	data, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		log.Printf("marshal trade message failed: %v", marshalErr)
	} else {
		worker.wsServer.Publish(worker.ctx, CurrentPriceTopic, data)
	}
}

func WsFanoutMarketDepth(worker *OrderWorker) {
	snapshot, err := worker.engine.Snapshot(book.MAX_SNAPSHOT_LEVELS)
	if err != nil {
		log.Printf("build market-depth snapshot failed: %v", err)
		return
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("marshal market-depth snapshot failed: %v", err)
		return
	}

	worker.wsServer.Publish(worker.ctx, MarketDepthTopic, data)
}

func WsFanoutTrades(plan matching.MatchPlan, worker *OrderWorker) {
	tradeMessages := make([]TradeMessage, 0, len(plan.Trades))
	for _, trade := range plan.Trades {
		tradeMessage := TradeMessage{
			TradeID:     trade.ID,
			Symbol:      trade.Symbol.Code,
			BuyOrderID:  trade.BuyOrderID,
			SellOrderID: trade.SellOrderID,
			Price:       trade.Price,
			Quantity:    trade.Quantity,
			CreatedAt:   trade.CreatedAt,
		}
		tradeMessages = append(tradeMessages, tradeMessage)
	}
	message := TradeMessages{
		Symbol: plan.IncomingOrder.Symbol.Code,
		Trades: tradeMessages,
	}
	data, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		log.Printf("marshal trade message failed: %v", marshalErr)
	} else {
		worker.wsServer.Publish(worker.ctx, TradesTopic, data)
	}
}
