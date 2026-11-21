package app

import (
	"context"
	"encoding/json"
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

	insertStartedSignal   chan bool
	startInsertOrder      chan bool
	updatedAsFailedSignal chan bool

	persistSubmissionStartedSignal chan bool
	persistSubmissionEndedSignal   chan bool
	startPersistSubmission         chan bool
	failPersistSubmission          bool

	failInsertOrder       bool
	insertSequenceAttempt chan int64
	persistedSequences    chan int64

	failUpdateOrder           bool
	isPersistSubmissionCalled bool
}

type publishCall struct {
	topic  WsTopic
	symbol string
	data   []byte
}

type fakeWsServer struct {
	calls           []publishCall
	publishedSignal chan struct{}
}

func (wsServer *fakeWsServer) Publish(ctx context.Context, topic WsTopic, symbol string, data []byte) {
	wsServer.calls = append(wsServer.calls, publishCall{
		topic:  topic,
		symbol: symbol,
		data:   append([]byte(nil), data...),
	})

	if wsServer.publishedSignal != nil {
		wsServer.publishedSignal <- struct{}{}
	}
}

func (store *fakeOrderWorkerStore) InsertOrder(ctx context.Context, order domain.Order) error {
	if store.insertStartedSignal != nil {
		store.insertStartedSignal <- true
	}
	if store.startInsertOrder != nil {
		<-store.startInsertOrder
	}
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
	for _, order := range orders {
		store.calls = append(store.calls, storeCall{
			method: "update",
			order:  order,
		})
		if order.Status == domain.StatusFailed {
			store.updatedAsFailedSignal <- true
		}
	}

	if store.failUpdateOrder {
		return ErrFakeUpdateErr
	}

	return nil
}

func (store *fakeOrderWorkerStore) PersistSubmission(ctx context.Context, incoming domain.Order, resting []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	defer func() {
		if store.persistSubmissionEndedSignal != nil {
			store.persistSubmissionEndedSignal <- true
		}
	}()

	store.isPersistSubmissionCalled = true
	if store.persistedSequences != nil {
		store.persistedSequences <- incoming.Sequence
	}

	if store.persistSubmissionStartedSignal != nil {
		store.persistSubmissionStartedSignal <- true
	}
	if store.startPersistSubmission != nil {
		<-store.startPersistSubmission
	}
	if store.failPersistSubmission {
		return errors.New("Failed by store.failPersisSubmission")
	}
	return nil
}

func getTestWorker(t *testing.T, symbolCode string, store orderWorkerStore, wsServer orderPublisher, queueCapacity int, participantIdNameMappings ...map[int64]string) *OrderWorker {
	t.Helper()

	symbol, err := domain.NewSymbol(symbolCode, 10)
	if err != nil {
		t.Fatalf("domain.NewSymbol failed: %v", err)
	}

	orderBook := book.NewBook(symbol, time.Now().UTC(), 30000)
	engine := matching.NewEngine(orderBook)

	participantIdNameMapping := map[int64]string{}
	if len(participantIdNameMappings) > 0 {
		participantIdNameMapping = participantIdNameMappings[0]
	}

	worker, err := NewOrderWorker(store, engine, symbol, wsServer, 1, queueCapacity, participantIdNameMapping)
	if err != nil {
		t.Fatalf("NewOrderWorker failed: %v", err)
	}
	t.Cleanup(worker.Stop)

	return worker
}

/*
Verify that a valid submission:
Receives lastSequence + 1.
Is inserted as PENDING.
Does not issue a separate pending-status update.
Returns nil.
*/
func TestOrderWorkerSubmitQueuesPendingOrder(t *testing.T) {
	symbolCode := "ASELS"
	store := &fakeOrderWorkerStore{}
	fakeWsServer := &fakeWsServer{}

	worker := getTestWorker(t, symbolCode, store, fakeWsServer, 10)
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

	if len(store.calls) != 1 {
		t.Fatalf("expected 1 storage call, got %d", len(store.calls))
	}

	insertCall := store.calls[0]
	if insertCall.method != "insert" {
		t.Fatalf("expected first call to be insert, got %s", insertCall.method)
	}

	if insertCall.order.Status != domain.StatusPending {
		t.Fatalf("expected inserted status PENDING, got %s", insertCall.order.Status)
	}
}

func TestOrderWorkerWaitsForInsertBeforeProcessing(t *testing.T) {
	store := &fakeOrderWorkerStore{
		insertStartedSignal:            make(chan bool),
		startInsertOrder:               make(chan bool),
		persistSubmissionStartedSignal: make(chan bool, 1),
	}
	worker := getTestWorker(t, "ASELS", store, &fakeWsServer{}, 10)
	t.Cleanup(func() { close(store.startInsertOrder) })
	request := SubmitOrderRequest{
		ParticipantId: 1, Symbol: "ASELS", Side: domain.SideBuy, Price: 30000, Quantity: 10,
	}
	submitDone := make(chan error, 1)
	go func() { submitDone <- worker.Submit(context.Background(), request) }()

	<-store.insertStartedSignal
	if len(worker.queue) != 0 {
		t.Fatal("order queued before insert completed")
	}
	select {
	case <-store.persistSubmissionStartedSignal:
		t.Fatal("processing started before insert completed")
	default:
	}

	store.startInsertOrder <- true
	if err := <-submitDone; err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	select {
	case <-store.persistSubmissionStartedSignal:
	case <-time.After(time.Second):
		t.Fatal("processing did not start after insert completed")
	}
	worker.Stop()
	if len(store.calls) != 1 || store.calls[0].order.Status != domain.StatusPending {
		t.Fatalf("expected only a PENDING insert, got %+v", store.calls)
	}
}

func TestOrderWorkerConcurrentSubmissionsPreserveSequence(t *testing.T) {
	const orderCount = 16
	store := &fakeOrderWorkerStore{persistedSequences: make(chan int64, orderCount)}
	worker := getTestWorker(t, "ASELS", store, &fakeWsServer{}, orderCount)
	initialNextSequence := worker.nextSequence
	request := SubmitOrderRequest{
		ParticipantId: 1, Symbol: "ASELS", Side: domain.SideBuy, Price: 30000, Quantity: 10,
	}
	submitDone := make(chan error, orderCount)
	for range orderCount {
		go func() { submitDone <- worker.Submit(context.Background(), request) }()
	}
	for range orderCount {
		if err := <-submitDone; err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}
	worker.Stop()
	if len(store.calls) != orderCount || len(store.persistedSequences) != orderCount {
		t.Fatalf("expected %d inserts and processed orders, got %d and %d", orderCount, len(store.calls), len(store.persistedSequences))
	}
	for index, call := range store.calls {
		expectedSequence := initialNextSequence + int64(index)
		processedSequence := <-store.persistedSequences
		if call.method != "insert" || call.order.Status != domain.StatusPending || call.order.Sequence != expectedSequence || processedSequence != expectedSequence {
			t.Fatalf("expected PENDING insert and processing at sequence %d, got %+v and %d", expectedSequence, call, processedSequence)
		}
	}
}

func TestOrderWorkerSkipsProcessingWhenInsertFails(t *testing.T) {
	store := &fakeOrderWorkerStore{failInsertOrder: true}
	worker := getTestWorker(t, "ASELS", store, &fakeWsServer{}, 10)
	request := SubmitOrderRequest{
		ParticipantId: 1, Symbol: "ASELS", Side: domain.SideBuy, Price: 30000, Quantity: 10,
	}
	if err := worker.Submit(context.Background(), request); err == nil {
		t.Fatal("expected insert error")
	}
	worker.Stop()
	if store.isPersistSubmissionCalled {
		t.Fatal("processing started after insert failed")
	}
	if len(store.calls) != 0 {
		t.Fatalf("expected no successful storage calls, got %+v", store.calls)
	}
}

/*
Block processing of the first order and use queue capacity 1:
first order  → worker blocked processing
second order → occupies queue
third order  → queue full
*/
func TestOrderWorkerRejectsWhenQueueIsFull(t *testing.T) {
	for _, failUpdateOrder := range []bool{false, true} {
		t.Run(fmt.Sprintf("update_failure=%t", failUpdateOrder), func(t *testing.T) {
			symbolCode := "ASELS"
			store := &fakeOrderWorkerStore{
				persistSubmissionStartedSignal: make(chan bool, 1),
				startPersistSubmission:         make(chan bool),
				failUpdateOrder:                failUpdateOrder,
			}
			fakeWsServer := &fakeWsServer{}

			worker := getTestWorker(t, symbolCode, store, fakeWsServer, 1)

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
			expectedErr := ErrSubmissionQueueFull
			if failUpdateOrder {
				expectedErr = ErrFakeUpdateErr
			}
			if !errors.Is(thirdSubmitErr, expectedErr) {
				t.Errorf("expected %v, got %v", expectedErr, thirdSubmitErr)
			}
			if len(store.calls) != 4 {
				t.Fatalf("expected three inserts and one rejection update, got %+v", store.calls)
			}
			for _, call := range store.calls[:3] {
				if call.method != "insert" || call.order.Status != domain.StatusPending {
					t.Fatalf("expected PENDING insert, got %+v", call)
				}
			}
			lastCall := store.calls[3]
			if lastCall.method != "update" || lastCall.order.Status != domain.StatusRejected {
				t.Fatalf("expected REJECTED update, got %+v", lastCall)
			}
			if lastCall.order.ID != store.calls[2].order.ID {
				t.Fatal("rejection update targeted wrong order")
			}
			if len(worker.queue) != 1 {
				t.Fatalf("expected only second order queued, got %d", len(worker.queue))
			}
		})
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
	fakeWsServer := &fakeWsServer{}

	symbolCode := "ASELS"
	worker := getTestWorker(t, symbolCode, store, fakeWsServer, 1)
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

func TestNewOrderWorkerCopiesPriceLimits(t *testing.T) {
	store := &fakeOrderWorkerStore{}
	fakeWsServer := &fakeWsServer{}
	worker := getTestWorker(t, "ASELS", store, fakeWsServer, 10)

	const expectedLowerPriceLimit int64 = 27000
	const expectedUpperPriceLimit int64 = 33000

	if worker.lowerPriceLimit != expectedLowerPriceLimit {
		t.Fatalf("expected lower price limit %d, got %d", expectedLowerPriceLimit, worker.lowerPriceLimit)
	}

	if worker.upperPriceLimit != expectedUpperPriceLimit {
		t.Fatalf("expected upper price limit %d, got %d", expectedUpperPriceLimit, worker.upperPriceLimit)
	}
}

func TestOrderWorkerRejectsPriceOutsideLimitsBeforeStorage(t *testing.T) {
	tests := []struct {
		name  string
		price int64
	}{
		{
			name:  "below lower price limit",
			price: 26990,
		},
		{
			name:  "above upper price limit",
			price: 33010,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeOrderWorkerStore{}
			fakeWsServer := &fakeWsServer{}

			worker := getTestWorker(t, "ASELS", store, fakeWsServer, 10)
			initialNextSequence := worker.nextSequence

			request := SubmitOrderRequest{
				ParticipantId: 1,
				Symbol:        "ASELS",
				Side:          domain.SideBuy,
				Price:         test.price,
				Quantity:      10,
			}

			submitErr := worker.Submit(context.Background(), request)
			if !errors.Is(submitErr, ErrInvalidOrder) {
				t.Fatalf("expected ErrInvalidOrder, got %v", submitErr)
			}

			if !errors.Is(submitErr, book.ErrPriceOutsideAllowedRange) {
				t.Fatalf("expected ErrPriceOutsideAllowedRange, got %v", submitErr)
			}

			if len(store.calls) != 0 {
				t.Fatalf("expected no storage calls, got %d", len(store.calls))
			}

			if worker.nextSequence != initialNextSequence {
				t.Fatalf("expected next sequence to remain %d, got %d", initialNextSequence, worker.nextSequence)
			}
		})
	}
}

func TestOrderWorkerAcceptsPricesAtLimits(t *testing.T) {
	tests := []struct {
		name  string
		price int64
	}{
		{
			name:  "lower price limit",
			price: 27000,
		},
		{
			name:  "upper price limit",
			price: 33000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeOrderWorkerStore{}
			fakeWsServer := &fakeWsServer{}
			worker := getTestWorker(t, "ASELS", store, fakeWsServer, 10)

			request := SubmitOrderRequest{
				ParticipantId: 1,
				Symbol:        "ASELS",
				Side:          domain.SideBuy,
				Price:         test.price,
				Quantity:      10,
			}

			if err := worker.Submit(context.Background(), request); err != nil {
				t.Fatalf("expected price at limit to be accepted, got %v", err)
			}

			if len(store.calls) != 1 {
				t.Fatalf("expected only insert storage call, got %d", len(store.calls))
			}
		})
	}
}

func TestOrderWorkerReturnsFailedAfterProccessStarts(t *testing.T) {
	store := &fakeOrderWorkerStore{
		failPersistSubmission:        true,
		persistSubmissionEndedSignal: make(chan bool),
		updatedAsFailedSignal:        make(chan bool),
	}

	symbolCode := "ASELS"
	fakeWsServer := &fakeWsServer{}
	worker := getTestWorker(t, symbolCode, store, fakeWsServer, 1)

	ctx := context.Background()
	request := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	requestErr := worker.Submit(ctx, request)
	if requestErr != nil {
		t.Fatalf("Expected no error on request submit, got: %v", requestErr)
	}

	<-store.persistSubmissionEndedSignal
	<-store.updatedAsFailedSignal

	call := store.calls[1]
	if call.method != "update" {
		t.Fatalf("Expected update method, got: %s", call.method)
	}

	if call.order.Status != domain.StatusFailed {
		t.Fatalf("Expected status failed, got: %v", call.order.Status)
	}
}

func waitForWsPublications(t *testing.T, publisher *fakeWsServer, count int) {
	t.Helper()

	for range count {
		<-publisher.publishedSignal
	}
}

func TestOrderWorkerWsFandout(t *testing.T) {
	store := &fakeOrderWorkerStore{}
	symbolCode := "ASELS"
	fakeWsServer := &fakeWsServer{
		calls:           make([]publishCall, 0),
		publishedSignal: make(chan struct{}, 4),
	}
	worker := getTestWorker(t, symbolCode, store, fakeWsServer, 1, map[int64]string{
		1: "GARANTII",
		3: "BOFFA",
	})

	ctx := context.Background()
	buyRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideBuy,
		Price:         30000,
		Quantity:      10,
	}

	buyRequestErr := worker.Submit(ctx, buyRequest)
	if buyRequestErr != nil {
		t.Fatalf("Expected no error on buyRequest submit, got: %v", buyRequestErr)
	}

	<-fakeWsServer.publishedSignal

	//WsFanoutMarketDepth
	firstCall := fakeWsServer.calls[0]
	if firstCall.topic != MarketDepthTopic {
		t.Fatalf("Expected first topic to be MarketDepthTopic, got: %v", firstCall.topic)
	}
	if firstCall.symbol != symbolCode {
		t.Fatalf("Expected first symbol to be %s, got: %s", symbolCode, firstCall.symbol)
	}

	var firstCallData book.Snapshot
	if err := json.Unmarshal(firstCall.data, &firstCallData); err != nil {
		t.Fatalf("Error occured on firstCallData json unmarshal: %v", err)
	}

	if firstCallData.Buy[0].Price != 30000 {
		t.Fatalf("Expected fanouted market depth first price to be 30000, got: %d", firstCallData.Buy[0].Price)
	}

	if firstCallData.Buy[0].Quantity != 10 {
		t.Fatalf("Expected fanouted market depth first qty to be 10, got: %d", firstCallData.Buy[0].Quantity)
	}

	sellRequest := SubmitOrderRequest{
		ParticipantId: 1,
		Symbol:        symbolCode,
		Side:          domain.SideSell,
		Price:         29500,
		Quantity:      15,
	}

	sellRequestErr := worker.Submit(ctx, sellRequest)
	if sellRequestErr != nil {
		t.Fatalf("Expected no error on sellRequest submit, got: %v", sellRequestErr)
	}

	waitForWsPublications(t, fakeWsServer, 3)

	//WsFanoutCurrentPrice
	secondCall := fakeWsServer.calls[1]
	if secondCall.topic != CurrentPriceTopic {
		t.Fatalf("Expected second topic to be CurrentPriceTopic, got: %v", secondCall.topic)
	}
	if secondCall.symbol != symbolCode {
		t.Fatalf("Expected second symbol to be %s, got: %s", symbolCode, secondCall.symbol)
	}

	var secondCallData LastTradePriceMessage
	if err := json.Unmarshal(secondCall.data, &secondCallData); err != nil {
		t.Fatalf("Error occured on secondCallData json unmarshal: %v", err)
	}

	if secondCallData.Price != 30000 {
		t.Fatalf("Expected last trade price on fanout to be 30000, got: %d", secondCallData.Price)
	}

	//WsFanoutTrades
	thirdCall := fakeWsServer.calls[2]
	if thirdCall.topic != TradesTopic {
		t.Fatalf("Expected third topic to be TradesTopic, got: %v", thirdCall.topic)
	}
	if thirdCall.symbol != symbolCode {
		t.Fatalf("Expected third symbol to be %s, got: %s", symbolCode, thirdCall.symbol)
	}

	var thirdCallData TradeMessages
	if err := json.Unmarshal(thirdCall.data, &thirdCallData); err != nil {
		t.Fatalf("Error occured on thirdCallData json unmarshal: %v", err)
	}

	if len(thirdCallData.Trades) != 1 {
		t.Fatalf("Expected 1 trade, got: %d", len(thirdCallData.Trades))
	}

	if thirdCallData.Trades[0].Quantity != 10 {
		t.Fatalf("Expected trade qty to be 10, got: %d", thirdCallData.Trades[0].Quantity)
	}

	//WsFanoutMarketDepth
	fourthCall := fakeWsServer.calls[3]
	if fourthCall.topic != MarketDepthTopic {
		t.Fatalf("Expected fourth topic to be MarketDepthTopic, got: %v", fourthCall.topic)
	}
	if fourthCall.symbol != symbolCode {
		t.Fatalf("Expected fourth symbol to be %s, got: %s", symbolCode, fourthCall.symbol)
	}

	var fourthCallData book.Snapshot
	if err := json.Unmarshal(fourthCall.data, &fourthCallData); err != nil {
		t.Fatalf("Error occured on fourthCallData json unmarshal: %v", err)
	}
	if fourthCallData.Sell[0].Quantity != 5 {
		t.Fatalf("Expected fourth fanouted market depth first qty to be 5, got: %d", fourthCallData.Sell[0].Quantity)
	}
}
