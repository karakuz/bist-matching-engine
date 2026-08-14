// internal/app/service_test.go
package app

import (
	"context"
	"os"
	"testing"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"

	"github.com/google/uuid"
)

func TestSubmitOrderPersistsFinalMatchResult(t *testing.T) {
	if os.Getenv("BME_PG_CONNSTRING") == "" {
		t.Skip("BME_PG_CONNSTRING is not set")
	}

	ctx := context.Background()

	pool, err := storage.NewPostgresPoolFromEnv(ctx)
	if err != nil {
		t.Fatalf("create postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)

	store := storage.NewPostgresStore(pool)

	initialization, err := store.GetBookInitialization(ctx, "ASELS")
	if err != nil {
		t.Fatalf("GetBookInitialization failed: %v", err)
	}

	orderBook := book.NewBook(
		initialization.Symbol,
		initialization.SessionDate,
		initialization.OpeningPrice,
	)
	engine := matching.NewEngine(orderBook)

	var participantID int64

	err = pool.QueryRow(
		ctx,
		`INSERT INTO participants (name, type_id) VALUES ($1, 2) RETURNING id`,
		uuid.NewString(),
	).Scan(&participantID)
	if err != nil {
		t.Fatalf("create test participant: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()

		if _, err := pool.Exec(
			cleanupCtx,
			`
				DELETE FROM trades
				WHERE buy_order_id IN (
					SELECT id FROM orders WHERE participant_id = $1
				)
				OR sell_order_id IN (
					SELECT id FROM orders WHERE participant_id = $1
				)
			`,
			participantID,
		); err != nil {
			t.Errorf("cleanup trades: %v", err)
		}

		if _, err := pool.Exec(
			cleanupCtx,
			`
				DELETE FROM order_events
				WHERE order_id IN (
					SELECT id
					FROM orders
					WHERE participant_id = $1
				)
			`,
			participantID,
		); err != nil {
			t.Errorf("cleanup order events: %v", err)
		}

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM orders WHERE participant_id = $1",
			participantID,
		); err != nil {
			t.Errorf("cleanup orders: %v", err)
		}

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM participants WHERE id = $1",
			participantID,
		); err != nil {
			t.Errorf("cleanup participant: %v", err)
		}
	})

	submitBuyOrderRequest := SubmitOrderRequest{
		ParticipantId: participantID,
		Symbol:        "ASELS",
		Side:          domain.SideBuy,
		Price:         35000,
		Quantity:      10,
	}

	submitSellOrderRequest := SubmitOrderRequest{
		ParticipantId: participantID,
		Symbol:        "ASELS",
		Side:          domain.SideSell,
		Price:         35000,
		Quantity:      4,
	}

	lastSequence, err := store.GetLastOrderSequence(
		ctx,
		initialization.Symbol.Code,
		initialization.SessionDate,
	)
	if err != nil {
		t.Fatalf("GetLastOrderSequence failed: %v", err)
	}

	worker, err := NewOrderWorker(
		store,
		engine,
		initialization.Symbol,
		lastSequence,
		10,
	)
	if err != nil {
		t.Fatalf("NewOrderWorker failed: %v", err)
	}
	t.Cleanup(worker.Stop)

	buyOrder, err := CreateOrderFromSubmitOrderRequest(worker, submitBuyOrderRequest)
	if err != nil{
		t.Fatalf("CreateOrderFromSubmitOrderRequest: %v", err)
	}

	_, buyOrderPlan, err := worker.process(ctx, buyOrder)
	if err != nil {
		t.Fatalf("process BUY: %v", err)
	}


	sellOrder, err := CreateOrderFromSubmitOrderRequest(worker, submitSellOrderRequest)
	if err != nil{
		t.Fatalf("CreateOrderFromSubmitOrderRequest: %v", err)
	}
	_, sellOrderPlan, err := worker.process(ctx, sellOrder)
	if err != nil {
		t.Fatalf("process SELL: %v", err)
	}

	var buySequence int64
	var sellSequence int64


	err = pool.QueryRow(
		ctx,
		`SELECT sequence_number FROM orders WHERE id = $1`,
		sellOrderPlan.IncomingOrder.ID,
	).Scan(&sellSequence)
	if err != nil {
		t.Fatalf("query SELL sequence: %v", err)
	}

	if buySequence != lastSequence+1 {
		t.Errorf("unexpected persisted BUY sequence: %d", buySequence)
	}

	if sellSequence != lastSequence+2 {
		t.Errorf("unexpected persisted SELL sequence: %d", sellSequence)
	}

	tradeID := sellOrderPlan.Trades[0].ID

	var buyStatus domain.OrderStatus
	var buyRemainingQuantity int64

	err = pool.QueryRow(
		ctx, `SELECT status, remaining_quantity FROM orders WHERE id = $1`,
		buyOrderPlan.IncomingOrder.ID,
	).Scan(&buyStatus, &buyRemainingQuantity)
	if err != nil {
		t.Fatalf("query BUY order: %v", err)
	}

	if buyStatus != domain.StatusPartiallyFilled {
		t.Errorf(
			"expected BUY status %s, got %s",
			domain.StatusPartiallyFilled,
			buyStatus,
		)
	}

	if buyRemainingQuantity != 6 {
		t.Errorf(
			"expected BUY remaining quantity 6, got %d",
			buyRemainingQuantity,
		)
	}

	// Verify the incoming SELL's final database state.
	var sellStatus domain.OrderStatus
	var sellRemainingQuantity int64

	err = pool.QueryRow(ctx, `SELECT status, remaining_quantity FROM orders WHERE id = $1`,
		sellOrderPlan.IncomingOrder.ID,
	).Scan(&sellStatus, &sellRemainingQuantity)
	if err != nil {
		t.Fatalf("query SELL order: %v", err)
	}

	if sellStatus != domain.StatusFilled {
		t.Errorf(
			"expected SELL status %s, got %s",
			domain.StatusFilled,
			sellStatus,
		)
	}

	if sellRemainingQuantity != 0 {
		t.Errorf(
			"expected SELL remaining quantity 0, got %d",
			sellRemainingQuantity,
		)
	}

	// Verify the persisted trade.
	var buyOrderID string
	var sellOrderIDFromDB string
	var tradePrice int64
	var tradeQuantity int64

	err = pool.QueryRow(ctx, `SELECT buy_order_id, sell_order_id, price, quantity FROM trades WHERE id = $1`,
		tradeID,
	).Scan(
		&buyOrderID,
		&sellOrderIDFromDB,
		&tradePrice,
		&tradeQuantity,
	)
	if err != nil {
		t.Fatalf("query trade: %v", err)
	}

	if buyOrderID != buyOrderPlan.IncomingOrder.ID {
		t.Errorf(
			"expected trade BUY order ID %s, got %s",
			buyOrderPlan.IncomingOrder.ID,
			buyOrderID,
		)
	}

	if sellOrderIDFromDB != sellOrderPlan.IncomingOrder.ID {
		t.Errorf(
			"expected trade SELL order ID %s, got %s",
			sellOrderPlan.IncomingOrder.ID,
			sellOrderIDFromDB,
		)
	}

	if tradePrice != 35000 {
		t.Errorf("expected trade price 35000, got %d", tradePrice)
	}

	if tradeQuantity != 4 {
		t.Errorf("expected trade quantity 4, got %d", tradeQuantity)
	}

	snapshot, err := engine.Snapshot(10)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if len(snapshot.Buy) != 1 {
		t.Fatalf("expected 1 BUY level, got %d", len(snapshot.Buy))
	}

	if snapshot.Buy[0].Price != 35000 {
		t.Errorf("expected BUY price 35000, got %d", snapshot.Buy[0].Price)
	}

	if snapshot.Buy[0].Quantity != 6 {
		t.Errorf("expected BUY quantity 6, got %d", snapshot.Buy[0].Quantity)
	}

	if len(snapshot.Sell) != 0 {
		t.Fatalf("expected no SELL levels, got %d", len(snapshot.Sell))
	}
}
