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

	buyResult, err := SubmitOrder(ctx, store, engine, submitBuyOrderRequest)
	if err != nil {
		t.Fatalf("submit BUY: %v", err)
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

	sellResult, err := SubmitOrder(ctx, store, engine, submitSellOrderRequest)
	if err != nil {
		t.Fatalf("submit SELL: %v", err)
	}

	if len(sellResult.Trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(sellResult.Trades))
	}

	tradeID := sellResult.Trades[0].ID

	var buyStatus domain.OrderStatus
	var buyRemainingQuantity int64

	err = pool.QueryRow(
		ctx, `SELECT status, remaining_quantity FROM orders WHERE id = $1`,
		buyResult.Order.ID,
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
		sellResult.Order.ID,
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

	if buyOrderID != buyResult.Order.ID {
		t.Errorf(
			"expected trade BUY order ID %s, got %s",
			buyResult.Order.ID,
			buyOrderID,
		)
	}

	if sellOrderIDFromDB != sellResult.Order.ID {
		t.Errorf(
			"expected trade SELL order ID %s, got %s",
			sellResult.Order.ID,
			sellOrderIDFromDB,
		)
	}

	if tradePrice != 35000 {
		t.Errorf("expected trade price 35000, got %d", tradePrice)
	}

	if tradeQuantity != 4 {
		t.Errorf("expected trade quantity 4, got %d", tradeQuantity)
	}
}
