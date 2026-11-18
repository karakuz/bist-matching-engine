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
	ready      chan bool
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
	stageStarted := time.Now()
	if err := request.Validate(worker); err != nil {
		log.Printf("order submit symbol=%s stage=validate duration=%s err=%v", request.Symbol, time.Since(stageStarted), err)
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	if !strings.EqualFold(request.Symbol, worker.symbol.Code) {
		log.Printf("order submit symbol=%s stage=validate duration=%s err=symbol does not match worker", request.Symbol, time.Since(stageStarted))
		return fmt.Errorf("%w: symbol does not match worker", ErrInvalidOrder)
	}

	request.Symbol = worker.symbol.Code
	log.Printf("order submit symbol=%s stage=validate duration=%s err=<nil>", request.Symbol, time.Since(stageStarted))

	lockStarted := time.Now()
	worker.submissionMutex.Lock()
	defer worker.submissionMutex.Unlock()
	log.Printf("order submit symbol=%s stage=submission_mutex_wait duration=%s", worker.symbol.Code, time.Since(lockStarted))

	if worker.stopped {
		log.Printf("order submit symbol=%s stage=submit_total duration=%s err=%v", worker.symbol.Code, time.Since(submitStarted), ErrWorkerStopped)
		return ErrWorkerStopped
	}

	stageStarted = time.Now()
	order, err := CreateOrderFromSubmitOrderRequest(worker, request)
	if err != nil {
		log.Printf("order submit symbol=%s stage=create_order duration=%s err=%v", worker.symbol.Code, time.Since(stageStarted), err)
		return fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}
	log.Printf("order=%s symbol=%s stage=create_order duration=%s err=<nil>", order.ID, worker.symbol.Code, time.Since(stageStarted))

	order.Sequence = worker.nextSequence

	stageStarted = time.Now()
	err = worker.store.InsertOrder(ctx, order)
	log.Printf("order=%s sequence=%d stage=insert_order duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
		log.Printf("Insert order failed: %v", err)
		log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), err)
		return err
	}

	worker.nextSequence++

	command := queueCommand{
		order:      &order,
		ready:      make(chan bool, 1),
		enqueuedAt: time.Now(),
	}

	var returnErr error = nil
	stageStarted = time.Now()
	select {
	case worker.queue <- command:
		order.Status = domain.StatusPending

	default:
		order.Status = domain.StatusRejected
		returnErr = ErrSubmissionQueueFull
	}
	log.Printf("order=%s sequence=%d stage=enqueue duration=%s queue_length=%d queue_capacity=%d rejected=%t", order.ID, order.Sequence, time.Since(stageStarted), len(worker.queue), cap(worker.queue), returnErr != nil)

	stageStarted = time.Now()
	err = worker.store.UpdateOrders(ctx, []domain.Order{order})
	log.Printf("order=%s sequence=%d stage=update_pending_order duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
		log.Printf("Update order failed: %v", err)
		stageStarted = time.Now()
		command.ready <- false
		log.Printf("order=%s sequence=%d stage=signal_worker_not_ready duration=%s", order.ID, order.Sequence, time.Since(stageStarted))
		log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), err)
		return err
	}
	stageStarted = time.Now()
	command.ready <- true
	log.Printf("order=%s sequence=%d stage=signal_worker_ready duration=%s", order.ID, order.Sequence, time.Since(stageStarted))
	log.Printf("order=%s sequence=%d stage=submit_total duration=%s err=%v", order.ID, order.Sequence, time.Since(submitStarted), returnErr)

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
		receivedAt := time.Now()
		readyWaitStarted := time.Now()
		ready := <-command.ready
		order := command.order
		log.Printf("order=%s sequence=%d stage=queue_wait duration=%s", order.ID, order.Sequence, receivedAt.Sub(command.enqueuedAt))
		log.Printf("order=%s sequence=%d stage=ready_signal_wait duration=%s", order.ID, order.Sequence, time.Since(readyWaitStarted))
		if !ready {
			log.Printf("Order with sequence %d is not ready, update have most likely failed, skipping", order.Sequence)
			continue
		}

		lockStarted := time.Now()
		worker.mutex.RLock()
		log.Printf("order=%s sequence=%d stage=worker_read_lock_wait duration=%s", order.ID, order.Sequence, time.Since(lockStarted))
		started := time.Now()

		_, plan, err := worker.process(worker.ctx, *order)
		if err != nil {
			log.Printf(
				"worker failed to process order id=%s sequence=%d: %v",
				order.ID,
				order.Sequence,
				err,
			)

			order.Status = domain.StatusFailed
			failureUpdateStarted := time.Now()
			err = worker.store.UpdateOrders(worker.ctx, []domain.Order{*order})
			log.Printf("order=%s sequence=%d stage=update_failed_order duration=%s err=%v", order.ID, order.Sequence, time.Since(failureUpdateStarted), err)
			if err != nil {
				log.Printf("[Worker Error] Update order failed: %v", err)
			}
		} else {
			//only fanout last trade price after trade
			if len(plan.Trades) > 0 {
				ltp := worker.engine.LastTradePrice()
				fanoutStarted := time.Now()
				WsFanoutCurrentPrice(ltp, worker)
				log.Printf("order=%s sequence=%d stage=fanout_current_price duration=%s", order.ID, order.Sequence, time.Since(fanoutStarted))
				fanoutStarted = time.Now()
				WsFanoutTrades(plan, worker)
				log.Printf("order=%s sequence=%d stage=fanout_trades duration=%s trades=%d", order.ID, order.Sequence, time.Since(fanoutStarted), len(plan.Trades))
			}

			fanoutStarted := time.Now()
			WsFanoutMarketDepth(worker)
			log.Printf("order=%s sequence=%d stage=fanout_market_depth duration=%s", order.ID, order.Sequence, time.Since(fanoutStarted))
		}

		worker.mutex.RUnlock()
		log.Printf("order=%s processing_time=%s err=%v", order.ID, time.Since(started), err)
	}
}

func (worker *OrderWorker) process(ctx context.Context, order domain.Order) ([]domain.OrderEvent, matching.MatchPlan, error) {
	if err := ctx.Err(); err != nil {
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	stageStarted := time.Now()
	plan, err := worker.engine.Prepare(order)
	log.Printf("order=%s sequence=%d stage=prepare duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	stageStarted = time.Now()
	events, err := buildSubmissionEvents(plan)
	log.Printf("order=%s sequence=%d stage=build_events duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
		err = fmt.Errorf("build submission events: %w", err)
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	stageStarted = time.Now()
	err = worker.store.PersistSubmission(
		ctx,
		plan.IncomingOrder,
		plan.RestingOrderUpdates,
		plan.Trades,
		events,
	)
	log.Printf("order=%s sequence=%d stage=persist_submission duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
		err = fmt.Errorf("persist submission: %w", err)
		return []domain.OrderEvent{}, matching.MatchPlan{}, err
	}

	stageStarted = time.Now()
	err = worker.engine.Apply(plan)
	log.Printf("order=%s sequence=%d stage=apply duration=%s err=%v", order.ID, order.Sequence, time.Since(stageStarted), err)
	if err != nil {
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
		Price:  ltp,
		Symbol: worker.symbol.Code,
	}
	started := time.Now()
	data, marshalErr := json.Marshal(message)
	log.Printf("ws_encode symbol=%s topic=%s stage=json_marshal duration=%s err=%v", worker.symbol.Code, CurrentPriceTopic, time.Since(started), marshalErr)
	if marshalErr != nil {
		log.Printf("marshal trade message failed: %v", marshalErr)
	} else {
		worker.wsServer.Publish(worker.ctx, CurrentPriceTopic, worker.symbol.Code, data)
	}
}

func WsFanoutMarketDepth(worker *OrderWorker) {
	started := time.Now()
	snapshot, err := worker.engine.Snapshot(book.MAX_SNAPSHOT_LEVELS)
	log.Printf("ws_encode symbol=%s topic=%s stage=snapshot duration=%s err=%v", worker.symbol.Code, MarketDepthTopic, time.Since(started), err)
	if err != nil {
		log.Printf("build market-depth snapshot failed: %v", err)
		return
	}

	started = time.Now()
	data, err := json.Marshal(snapshot)
	log.Printf("ws_encode symbol=%s topic=%s stage=json_marshal duration=%s bytes=%d err=%v", worker.symbol.Code, MarketDepthTopic, time.Since(started), len(data), err)
	if err != nil {
		log.Printf("marshal market-depth snapshot failed: %v", err)
		return
	}

	worker.wsServer.Publish(worker.ctx, MarketDepthTopic, worker.symbol.Code, data)
}

func WsFanoutTrades(plan matching.MatchPlan, worker *OrderWorker) {
	started := time.Now()
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
	log.Printf("ws_encode symbol=%s topic=%s stage=build_trade_messages duration=%s trades=%d", worker.symbol.Code, TradesTopic, time.Since(started), len(tradeMessages))
	message := TradeMessages{
		Symbol: plan.IncomingOrder.Symbol.Code,
		Trades: tradeMessages,
	}
	started = time.Now()
	data, marshalErr := json.Marshal(message)
	log.Printf("ws_encode symbol=%s topic=%s stage=json_marshal duration=%s bytes=%d err=%v", worker.symbol.Code, TradesTopic, time.Since(started), len(data), marshalErr)
	if marshalErr != nil {
		log.Printf("marshal trade message failed: %v", marshalErr)
	} else {
		worker.wsServer.Publish(worker.ctx, TradesTopic, worker.symbol.Code, data)
	}
}
