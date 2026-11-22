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
	"bist-matching-engine/internal/storage"

	"github.com/google/uuid"
)

var (
	ErrSubmissionQueueFull = errors.New("submission queue is full")
	ErrWorkerStopped       = errors.New("order worker is stopped")
	ErrApplyCommittedPlan  = errors.New("failed to apply committed match plan")
)

type orderWorkerStore interface {
	InsertOrder(context.Context, domain.Order) error
	UpdateOrders(context.Context, []domain.Order) error
	PersistSubmission(context.Context, domain.Order, []domain.Order, []domain.Trade, []domain.OrderEvent) error
}

type orderPublisher interface {
	Publish(context.Context, WsTopic, string, []byte)
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

	participantIdNameMapping map[int64]string
}

type queueCommand struct {
	order      *domain.Order
	enqueuedAt time.Time
}

func NewOrderWorker(
	store orderWorkerStore,
	engine *matching.Engine,
	symbol domain.Symbol,
	wsServer orderPublisher,
	lastSequence int64,
	queueCapacity int,
	participantIdNameMapping map[int64]string,
) (*OrderWorker, error) {
	if queueCapacity <= 0 {
		return nil, errors.New("queue capacity must be > 0")
	}

	lowerPriceLimit, upperPriceLimit := engine.PriceLimits()

	worker := &OrderWorker{
		ctx:                      context.Background(),
		store:                    store,
		engine:                   engine,
		symbol:                   symbol,
		wsServer:                 wsServer,
		queue:                    make(chan queueCommand, queueCapacity),
		nextSequence:             lastSequence + 1,
		done:                     make(chan struct{}),
		lowerPriceLimit:          lowerPriceLimit,
		upperPriceLimit:          upperPriceLimit,
		participantIdNameMapping: participantIdNameMapping,
	}

	go worker.run()

	return worker, nil
}

func (worker *OrderWorker) Submit(ctx context.Context, request SubmitOrderRequest) error {
	submitStarted := time.Now()
	if err := request.Validate(worker); err != nil {
		log.Printf("order submit symbol=%s validation failed: %v", request.Symbol, err)
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	if !strings.EqualFold(request.Symbol, worker.symbol.Code) {
		log.Printf("order submit symbol=%s: symbol does not match worker", request.Symbol)
		return fmt.Errorf("%w: symbol does not match worker", ErrInvalidOrder)
	}

	request.Symbol = worker.symbol.Code

	worker.submissionMutex.Lock()
	defer worker.submissionMutex.Unlock()

	if worker.stopped {
		log.Printf("order submit symbol=%s stage=submit_total duration=%s err=%v", worker.symbol.Code, time.Since(submitStarted), ErrWorkerStopped)
		return ErrWorkerStopped
	}

	order, err := CreateOrderFromSubmitOrderRequest(worker, request)
	if err != nil {
		log.Printf("order submit symbol=%s create order failed: %v", worker.symbol.Code, err)
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	order.Sequence = worker.nextSequence
	order.Status = domain.StatusPending

	err = worker.store.InsertOrder(ctx, order)
	if err != nil {
		log.Printf("Insert order failed: %v", err)
		log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), err)
		return err
	}

	worker.nextSequence++

	command := queueCommand{
		order:      &order,
		enqueuedAt: time.Now(),
	}

	var returnErr error = nil
	select {
	case worker.queue <- command:
	default:
		order.Status = domain.StatusRejected
		returnErr = ErrSubmissionQueueFull
	}

	if returnErr != nil {
		err = worker.store.UpdateOrders(ctx, []domain.Order{order})
		if err != nil {
			log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), err)
			return err
		}
	}
	log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), returnErr)

	return returnErr
}

func (worker *OrderWorker) Stop() {
	worker.submissionMutex.Lock()

	if !worker.stopped {
		worker.stopped = true
		close(worker.queue)
	}

	worker.submissionMutex.Unlock()

	<-worker.done
}

func (worker *OrderWorker) run() {
	defer close(worker.done)

	for command := range worker.queue {
		receivedAt := time.Now()
		order := command.order
		queueWait := receivedAt.Sub(command.enqueuedAt)

		worker.mutex.RLock()
		started := time.Now()

		_, plan, err := worker.process(worker.ctx, *order)
		var commitErr *storage.CommitOutcomeUnknownError
		if errors.Is(err, ErrApplyCommittedPlan) || errors.As(err, &commitErr) {
			log.Printf("worker stopped: symbol=%s order=%s sequence=%d; cannot safely continue: %v", worker.symbol.Code, order.ID, order.Sequence, err)
			worker.submissionMutex.Lock()
			if !worker.stopped {
				worker.stopped = true
				close(worker.queue)
			}
			worker.submissionMutex.Unlock()
			worker.mutex.RUnlock()
			return
		}
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
		log.Printf("order=%s sequence=%d queue_wait=%s processing_time=%s err=%v", order.ID, order.Sequence, queueWait, time.Since(started), err)
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

	err = worker.engine.Apply(plan)
	if err != nil {
		return events, plan, fmt.Errorf("%w: %w", ErrApplyCommittedPlan, err)
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
		Price:  ltp,
		Symbol: worker.symbol.Code,
	}
	data, marshalErr := json.Marshal(message)
	if marshalErr != nil {
		log.Printf("marshal trade message failed: %v", marshalErr)
	} else {
		worker.wsServer.Publish(worker.ctx, CurrentPriceTopic, worker.symbol.Code, data)
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

	worker.wsServer.Publish(worker.ctx, MarketDepthTopic, worker.symbol.Code, data)
}

func WsFanoutTrades(plan matching.MatchPlan, worker *OrderWorker) {
	tradeMessages := make([]TradeMessage, 0, len(plan.Trades))

	participantsByOrderID := make(map[string]int64, len(plan.RestingOrderUpdates)+1)
	participantsByOrderID[plan.IncomingOrder.ID] = plan.IncomingOrder.ParticipantID

	for _, restingOrder := range plan.RestingOrderUpdates {
		participantsByOrderID[restingOrder.ID] = restingOrder.ParticipantID
	}

	for _, trade := range plan.Trades {
		buyParticipantID := participantsByOrderID[trade.BuyOrderID]
		buyParticipantName := worker.participantIdNameMapping[buyParticipantID]

		sellParticipantID := participantsByOrderID[trade.SellOrderID]
		sellParticipantName := worker.participantIdNameMapping[sellParticipantID]

		tradeMessage := TradeMessage{
			TradeID:             trade.ID,
			Symbol:              trade.Symbol.Code,
			BuyOrderID:          trade.BuyOrderID,
			BuyParticipantID:    buyParticipantID,
			BuyParticipantName:  buyParticipantName,
			SellOrderID:         trade.SellOrderID,
			SellParticipantID:   sellParticipantID,
			SellParticipantName: sellParticipantName,
			Price:               trade.Price,
			Quantity:            trade.Quantity,
			CreatedAt:           trade.CreatedAt,
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
		worker.wsServer.Publish(worker.ctx, TradesTopic, worker.symbol.Code, data)
	}
}
