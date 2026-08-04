package app

import (
	"context"
	"errors"
	"fmt"
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

type submissionPersister interface {
	PersistSubmission(context.Context, domain.Order, []domain.Order, []domain.Trade, []domain.OrderEvent) error
}

type submissionCommand struct {
	ctx      context.Context
	request  SubmitOrderRequest
	response chan submissionResponse
}

type submissionResponse struct {
	result SubmitOrderResult
	err    error
}

type OrderWorker struct {
	store        submissionPersister
	engine       *matching.Engine
	symbol       domain.Symbol
	commands     chan submissionCommand
	nextSequence int64

	mutex   sync.RWMutex
	stopped bool
	done    chan struct{}
}

func NewOrderWorker(store submissionPersister, engine *matching.Engine, symbol domain.Symbol, lastSequence int64, queueCapacity int) (*OrderWorker, error) {
	if queueCapacity <= 0 {
		return nil, errors.New("queue capacity must be > 0")
	}

	worker := &OrderWorker{
		store:        store,
		engine:       engine,
		symbol:       symbol,
		commands:     make(chan submissionCommand, queueCapacity),
		nextSequence: lastSequence + 1,
		done:         make(chan struct{}),
	}

	go worker.run()

	return worker, nil
}

func (worker *OrderWorker) Submit(ctx context.Context, request SubmitOrderRequest) (SubmitOrderResult, error) {
	if err := request.Validate(); err != nil {
		return SubmitOrderResult{}, fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	if !strings.EqualFold(request.Symbol, worker.symbol.Code) {
		return SubmitOrderResult{}, fmt.Errorf("%w: symbol does not match worker", ErrInvalidOrder)
	}

	request.Symbol = worker.symbol.Code

	command := submissionCommand{
		ctx:      ctx,
		request:  request,
		response: make(chan submissionResponse, 1),
	}

	worker.mutex.RLock()

	if worker.stopped {
		worker.mutex.RUnlock()
		return SubmitOrderResult{}, ErrWorkerStopped
	}

	select {
		case worker.commands <- command:
			worker.mutex.RUnlock()

		default:
			worker.mutex.RUnlock()
			return SubmitOrderResult{}, ErrSubmissionQueueFull
	}

	select {
		case response := <-command.response:
			return response.result, response.err

		case <-ctx.Done():
			return SubmitOrderResult{}, ctx.Err()
	}
}

func (worker *OrderWorker) Stop() {
	worker.mutex.Lock()

	if !worker.stopped {
		worker.stopped = true
		close(worker.commands)
	}

	worker.mutex.Unlock()

	<-worker.done
}

func (worker *OrderWorker) run() {
	defer close(worker.done)

	for command := range worker.commands {
		result, err := worker.process(
			command.ctx,
			command.request,
		)

		command.response <- submissionResponse{
			result: result,
			err:    err,
		}
	}
}

func (worker *OrderWorker) process(ctx context.Context, request SubmitOrderRequest) (SubmitOrderResult, error) {
	if err := ctx.Err(); err != nil {
		return SubmitOrderResult{}, err
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		request.ParticipantId,
		worker.symbol,
		worker.engine.SessionDate(),
		request.Side,
		request.Price,
		request.Quantity,
		time.Now().UTC(),
	)
	if err != nil {
		return SubmitOrderResult{}, fmt.Errorf("%w: %w", ErrInvalidOrder, err)
	}

	order.Sequence = worker.nextSequence

	plan, err := worker.engine.Prepare(order)
	if err != nil {
		return SubmitOrderResult{}, err
	}

	events, err := buildSubmissionEvents(plan)
	if err != nil {
		return SubmitOrderResult{}, fmt.Errorf("build submission events: %w", err)
	}

	err = worker.store.PersistSubmission(
		ctx,
		plan.IncomingOrder,
		plan.RestingOrderUpdates,
		plan.Trades,
		events,
	)
	if err != nil {
		return SubmitOrderResult{}, fmt.Errorf("persist submission: %w", err)
	}

	// The sequence is persisted, so it must never be reused.
	worker.nextSequence++

	if err := worker.engine.Apply(plan); err != nil {
		return SubmitOrderResult{}, fmt.Errorf("apply persisted match plan: %w", err)
	}

	return SubmitOrderResult{
		Order:  plan.IncomingOrder,
		Trades: plan.Trades,
	}, nil
}
