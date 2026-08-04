package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
)

type capturedSubmission struct {
	incomingOrder domain.Order
}

type fakeSubmissionStore struct {
	mutex       sync.Mutex
	submissions []capturedSubmission
	entered     chan struct{}
	release     chan struct{}
	enterOnce   sync.Once
}

func (store *fakeSubmissionStore) PersistSubmission(ctx context.Context, incomingOrder domain.Order, restingUpdates []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	if store.entered != nil {
		store.enterOnce.Do(func() {
			close(store.entered)
		})
	}

	if store.release != nil {
		select {
		case <-store.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	store.mutex.Lock()
	store.submissions = append(
		store.submissions,
		capturedSubmission{incomingOrder: incomingOrder},
	)
	store.mutex.Unlock()

	return nil
}

func newTestWorker(t *testing.T, store submissionPersister, lastSequence int64, queueCapacity int) *OrderWorker {
	t.Helper()

	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	orderBook := book.NewBook(
		symbol,
		time.Now().UTC(),
		300,
	)

	worker, err := NewOrderWorker(
		store,
		matching.NewEngine(orderBook),
		symbol,
		lastSequence,
		queueCapacity,
	)
	if err != nil {
		t.Fatalf("NewOrderWorker failed: %v", err)
	}

	t.Cleanup(worker.Stop)

	return worker
}

func testSubmitRequest() SubmitOrderRequest {
	return SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        "ASELS",
		Side:          domain.SideBuy,
		Price:         300,
		Quantity:      10,
	}
}

func TestOrderWorkerAssignsIncreasingSequences(t *testing.T) {
	store := &fakeSubmissionStore{}
	worker := newTestWorker(t, store, 41, 10)

	firstResult, err := worker.Submit(
		context.Background(),
		testSubmitRequest(),
	)
	if err != nil {
		t.Fatalf("first Submit failed: %v", err)
	}

	secondResult, err := worker.Submit(
		context.Background(),
		testSubmitRequest(),
	)
	if err != nil {
		t.Fatalf("second Submit failed: %v", err)
	}

	if firstResult.Order.Sequence != 42 {
		t.Fatalf("expected first sequence 42, got %d", firstResult.Order.Sequence)
	}

	if secondResult.Order.Sequence != 43 {
		t.Fatalf("expected second sequence 43, got %d", secondResult.Order.Sequence)
	}
}

func TestOrderWorkerRejectsWhenQueueIsFull(t *testing.T) {
	store := &fakeSubmissionStore{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}

	worker := newTestWorker(t, store, 0, 1)

	firstDone := make(chan error, 1)

	go func() {
		_, err := worker.Submit(
			context.Background(),
			testSubmitRequest(),
		)
		firstDone <- err
	}()

	<-store.entered

	secondDone := make(chan error, 1)

	go func() {
		_, err := worker.Submit(
			context.Background(),
			testSubmitRequest(),
		)
		secondDone <- err
	}()

	deadline := time.Now().Add(time.Second)

	for len(worker.commands) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("second command was not queued")
		}

		time.Sleep(time.Millisecond)
	}

	_, err := worker.Submit(
		context.Background(),
		testSubmitRequest(),
	)
	if !errors.Is(err, ErrSubmissionQueueFull) {
		t.Fatalf("expected ErrSubmissionQueueFull, got %v", err)
	}

	close(store.release)

	if err := <-firstDone; err != nil {
		t.Fatalf("first submission failed: %v", err)
	}

	if err := <-secondDone; err != nil {
		t.Fatalf("second submission failed: %v", err)
	}
}
