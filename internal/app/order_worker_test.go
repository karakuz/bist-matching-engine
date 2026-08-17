package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
)

var (
	ErrFakeUpdateErr = errors.New("Fake update error")
)

type storeCall struct {
	method string
	order  domain.Order
}

type fakeOrderWorkerStore struct {
	calls []storeCall

	updateStartedSignal chan bool
	updateSignal        chan bool
	updatedSignal       chan bool

	persistSubmissionStartedSignal chan bool
	startPersistSubmission         chan bool

	failInsertOrder       bool
	insertSequenceAttempt chan int64

	failUpdateOrder           bool
	isPersistSubmissionCalled bool
}

func (store *fakeOrderWorkerStore) InsertOrder(ctx context.Context, order domain.Order) error {
	defer func() {
		if store.insertSequenceAttempt != nil {
			store.insertSequenceAttempt <- order.Sequence
		}
	}()

	if store.failInsertOrder {
		return fmt.Errorf("Insert order failed by store.failInsertOrder")
	}

	store.calls = append(store.calls, storeCall{
		method: "insert",
		order:  order,
	})
	return nil
}

func (store *fakeOrderWorkerStore) UpdateOrders(ctx context.Context, orders []domain.Order) error {
	if store.updateStartedSignal != nil {
		store.updateStartedSignal <- true
	}

	if store.updateSignal != nil {
		<-store.updateSignal
	}

	for _, order := range orders {
		store.calls = append(store.calls, storeCall{
			method: "update",
			order:  order,
		})
	}

	if store.failUpdateOrder {
		return ErrFakeUpdateErr
	}

	if store.updatedSignal != nil {
		store.updatedSignal <- true
	}

	return nil
}

func (store *fakeOrderWorkerStore) PersistSubmission(ctx context.Context, incoming domain.Order, resting []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	store.isPersistSubmissionCalled = true

	if store.persistSubmissionStartedSignal != nil {
		store.persistSubmissionStartedSignal <- true
	}
	if store.startPersistSubmission != nil {
		<-store.startPersistSubmission
	}
	return nil
}

func getTestWorker(t *testing.T, symbolCode string, store orderWorkerStore, queueCapacity int) *OrderWorker {
	t.Helper()

	symbol, err := domain.NewSymbol(symbolCode, 10)
	if err != nil {
		t.Fatalf("domain.NewSymbol failed: %v", err)
	}

	orderBook := book.NewBook(symbol, time.Now().UTC(), 30000)
	engine := matching.NewEngine(orderBook)

	worker, err := NewOrderWorker(store, engine, symbol, 1, queueCapacity)
	if err != nil {
		t.Fatalf("NewOrderWorker failed: %v", err)
	}
	t.Cleanup(worker.Stop)

	return worker
}

/*
Verify that a valid submission:
Receives lastSequence + 1.
Is first inserted as CREATED.
Is updated to PENDING after enqueue succeeds.
Returns nil.
*/
func TestOrderWorkerSubmitQueuesPendingOrder(t *testing.T) {
	symbolCode := "ASELS"
	store := &fakeOrderWorkerStore{}

	worker := getTestWorker(t, symbolCode, store, 10)
	ctx := context.Background()
	request := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}
	initialNextSequence := worker.nextSequence

	submitErr := worker.Submit(ctx, request)
	if submitErr != nil {
		t.Fatalf("worker.Submit failed: %v", submitErr)
	}

	currentNextSequence := worker.nextSequence
	if initialNextSequence+1 != currentNextSequence {
		t.Fatalf("expected initialNextSequence+1 = currentNextSequence; initialNextSequence: %d, currentNextSequence: %d", initialNextSequence, currentNextSequence)
	}

	if len(store.calls) != 2 {
		t.Fatalf("expected 2 storage calls, got %d", len(store.calls))
	}

	insertCall := store.calls[0]
	if insertCall.method != "insert" {
		t.Fatalf("expected first call to be insert, got %s", insertCall.method)
	}

	if insertCall.order.Status != domain.StatusCreated {
		t.Fatalf("expected inserted status CREATED, got %s", insertCall.order.Status)
	}

	updateCall := store.calls[1]
	if updateCall.method != "update" {
		t.Fatalf("expected second call to be update, got %s", updateCall.method)
	}

	if updateCall.order.Status != domain.StatusPending {
		t.Fatalf("expected updated status PENDING, got %s", updateCall.order.Status)
	}

}

/*
Block UpdateOrders in the fake store and verify PersistSubmission has not started.
Release UpdateOrders, then verify processing begins.
This directly tests the ready channel.
*/
func TestOrderWorkerWaitsForPendingUpdateBeforeProcessing(t *testing.T) {
	symbolCode := "ASELS"
	store := &fakeOrderWorkerStore{
		updateStartedSignal:       make(chan bool),
		updateSignal:              make(chan bool),
		updatedSignal:             make(chan bool),
		isPersistSubmissionCalled: false,
	}

	worker := getTestWorker(t, symbolCode, store, 10)
	ctx := context.Background()
	request := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	submitDone := make(chan error, 1)

	go func() {
		submitDone <- worker.Submit(ctx, request)
	}()

	<-store.updateStartedSignal
	if len(store.calls) != 1 {
		t.Fatalf("expected len(store.calls) == 1, got: %d", len(store.calls))
	}

	firstCall := store.calls[0]
	if firstCall.method != "insert" {
		t.Fatalf("Expected first call's method to be 'insert', got: %s", firstCall.method)
	}

	if store.isPersistSubmissionCalled != false {
		t.Fatalf("Expected persist submission not to be called")
	}

	store.updateSignal <- true
	<-store.updatedSignal
	submitErr := <-submitDone
	if submitErr != nil {
		t.Fatalf("submit failed: %v", submitErr)
	}

	if len(store.calls) != 2 {
		t.Fatalf("After update signal, expected len(store.calls) == 2, got: %d", len(store.calls))
	}

	secondCall := store.calls[1]
	if secondCall.method != "update" {
		t.Fatalf("Expected second call's method to be 'update', got: %s", secondCall.method)
	}

}

/*
Make UpdateOrders fail after successful enqueue. Verify:
Submit returns the database error.
The worker receives false.
PersistSubmission is never called.
*/
func TestOrderWorkerSkipsProcessingWhenPendingUpdateFails(t *testing.T) {
	symbolCode := "ASELS"
	store := &fakeOrderWorkerStore{
		failUpdateOrder:           true,
		isPersistSubmissionCalled: false,
	}

	worker := getTestWorker(t, symbolCode, store, 10)
	ctx := context.Background()
	request := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	submitErr := worker.Submit(ctx, request)
	if !errors.Is(submitErr, ErrFakeUpdateErr) {
		t.Fatalf("Expected submitErr to be equal to ErrFakeUpdateErr")
	}

	worker.Stop()

	if store.isPersistSubmissionCalled {
		t.Fatalf("Expected persist submission not been called, got: %t", store.isPersistSubmissionCalled)
	}
}

/*
Block processing of the first order and use queue capacity 1:
first order  → worker blocked processing
second order → occupies queue
third order  → queue full
*/
func TestOrderWorkerRejectsWhenQueueIsFull(t *testing.T) {
	symbolCode := "ASELS"
	store := &fakeOrderWorkerStore{
		persistSubmissionStartedSignal: make(chan bool, 1),
		startPersistSubmission:         make(chan bool),
	}

	worker := getTestWorker(t, symbolCode, store, 1)

	t.Cleanup(func() {
		close(store.startPersistSubmission)
	})

	ctx := context.Background()
	firstRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	err := worker.Submit(ctx, firstRequest)
	if err != nil {
		t.Fatalf("Expected no error on first submit")
	}

	<-store.persistSubmissionStartedSignal

	secondRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	secondSubmitErr := worker.Submit(ctx, secondRequest)
	if secondSubmitErr != nil {
		t.Errorf("expected no error on second submit, got %v", secondSubmitErr)
	}

	thirdRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	thirdSubmitErr := worker.Submit(ctx, thirdRequest)
	if !errors.Is(thirdSubmitErr, ErrSubmissionQueueFull) {
		t.Errorf("expected ErrSubmissionQueueFull, got %v", thirdSubmitErr)
	}
}

/*
Make the first InsertOrder fail, then submit another order successfully.
Verify the successful order receives the sequence that the failed insert attempted.
*/
func TestOrderWorkerDoesNotConsumeSequenceWhenInsertFails(t *testing.T) {
	store := &fakeOrderWorkerStore{
		failInsertOrder:       true,
		insertSequenceAttempt: make(chan int64, 1),
	}

	symbolCode := "ASELS"
	worker := getTestWorker(t, symbolCode, store, 1)
	ctx := context.Background()
	firstRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	firstRequestErr := worker.Submit(ctx, firstRequest)
	if firstRequestErr == nil {
		t.Fatalf("Expected error on first request submit, got: %v", firstRequestErr)
	}

	firstAttemptedSequenceNumber := <-store.insertSequenceAttempt

	store.failInsertOrder = false
	secondRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	err := worker.Submit(ctx, secondRequest)
	if err != nil {
		t.Fatalf("Expected no error on second submit, got: %v", err)
	}

	secondAttemptedSequenceNumber := <-store.insertSequenceAttempt
	if firstAttemptedSequenceNumber != secondAttemptedSequenceNumber {
		t.Fatalf("Expected firstAttemptedSequenceNumber == secondAttemptedSequenceNumber, got false")
	}
}
