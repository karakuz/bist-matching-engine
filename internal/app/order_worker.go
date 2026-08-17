package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

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

type OrderWorker struct {
	ctx             context.Context
	store           orderWorkerStore
	engine          *matching.Engine
	symbol          domain.Symbol
	queue           chan queueCommand
	submissionMutex sync.Mutex
	nextSequence    int64

	mutex   sync.RWMutex
	stopped bool
	done    chan struct{}
}

type queueCommand struct {
	order *domain.Order
	ready chan bool
}

func NewOrderWorker(store orderWorkerStore, engine *matching.Engine, symbol domain.Symbol, lastSequence int64, queueCapacity int) (*OrderWorker, error) {
	if queueCapacity <= 0 {
		return nil, errors.New("queue capacity must be > 0")
	}

	worker := &OrderWorker{
		ctx:          context.Background(),
		store:        store,
		engine:       engine,
		symbol:       symbol,
		queue:        make(chan queueCommand, queueCapacity),
		nextSequence: lastSequence + 1,
		done:         make(chan struct{}),
	}

	go worker.run()

	return worker, nil
}

func (worker *OrderWorker) Submit(ctx context.Context, request SubmitOrderRequest) error {
	if err := request.Validate(); err != nil {
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

		_, _, err := worker.process(worker.ctx, *order)
		if err != nil {
			log.Printf("Error occured on worker, order:  %v", order)
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
