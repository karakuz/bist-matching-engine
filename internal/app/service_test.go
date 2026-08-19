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

	if err := worker.Submit(ctx, submitBuyOrderRequest); err != nil {
		t.Fatalf("Submit BUY: %v", err)
	}

	if err := worker.Submit(ctx, submitSellOrderRequest); err != nil {
		t.Fatalf("Submit SELL: %v", err)
	}

	worker.Stop()

	var buyOrderID string
	var buySequence int64
	var buyStatus domain.OrderStatus
	var buyRemainingQuantity int64

	err = pool.QueryRow(
		ctx,
		`SELECT id, sequence_number, status, remaining_quantity FROM orders WHERE participant_id = $1 AND side = $2`,
		participantID,
		domain.SideBuy,
	).Scan(&buyOrderID, &buySequence, &buyStatus, &buyRemainingQuantity)
	if err != nil {
		t.Fatalf("query BUY order: %v", err)
	}

	var sellOrderID string
	var sellSequence int64
	var sellStatus domain.OrderStatus
	var sellRemainingQuantity int64

	err = pool.QueryRow(
		ctx,
		`SELECT id, sequence_number, status, remaining_quantity FROM orders WHERE participant_id = $1 AND side = $2`,
		participantID,
		domain.SideSell,
	).Scan(&sellOrderID, &sellSequence, &sellStatus, &sellRemainingQuantity)
	if err != nil {
		t.Fatalf("query SELL order: %v", err)
	}

	if buySequence != lastSequence+1 {
		t.Errorf("unexpected persisted BUY sequence: %d", buySequence)
	}

	if sellSequence != lastSequence+2 {
		t.Errorf("unexpected persisted SELL sequence: %d", sellSequence)
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
	var sellOrderIDFromDB string
	var tradePrice int64
	var tradeQuantity int64

	err = pool.QueryRow(ctx, `SELECT sell_order_id, price, quantity FROM trades WHERE buy_order_id = $1`,
		buyOrderID,
	).Scan(
		&sellOrderIDFromDB,
		&tradePrice,
		&tradeQuantity,
	)
	if err != nil {
		t.Fatalf("query trade: %v", err)
	}

	if sellOrderIDFromDB != sellOrderID {
		t.Errorf(
			"expected trade SELL order ID %s, got %s",
			sellOrderID,
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
