package storage

import (
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
)

func TestPersistSubmissionRollsBackWhenTradeInsertFails(t *testing.T) {
	store, ctx := newTestStore(t)

	initialization, err := store.GetBookInitialization(ctx, "ASELS")
	if err != nil {
		t.Fatalf("GetBookInitialization failed: %v", err)
	}

	var participantID int64

	err = store.pool.QueryRow(
		ctx,
		`SELECT id FROM participants ORDER BY id LIMIT 1`,
	).Scan(&participantID)
	if err != nil {
		t.Fatalf("get participant: %v", err)
	}

	lastSequence, err := store.GetLastOrderSequence(
		ctx,
		initialization.Symbol.Code,
		initialization.SessionDate,
	)
	if err != nil {
		t.Fatalf("GetLastOrderSequence failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		participantID,
		initialization.Symbol,
		initialization.SessionDate,
		domain.SideBuy,
		initialization.OpeningPrice,
		10,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	order.Sequence = lastSequence + 1

	if err := store.InsertOrder(ctx, order); err != nil {
		t.Fatalf("InsertOrder failed: %v", err)
	}

	t.Cleanup(func() {
		if _, err := store.pool.Exec(ctx, `DELETE FROM trades WHERE buy_order_id = $1`, order.ID); err != nil {
			t.Errorf("cleanup trades: %v", err)
		}

		if _, err := store.pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, order.ID); err != nil {
			t.Errorf("cleanup order: %v", err)
		}
	})

	finalOrder := order
	finalOrder.RemainingQuantity = 9
	finalOrder.Status = domain.StatusPartiallyFilled

	duplicateTradeID := uuid.NewString()

	trades := []domain.Trade{
		{
			ID:          duplicateTradeID,
			Symbol:      initialization.Symbol,
			BuyOrderID:  order.ID,
			SellOrderID: "sell-1",
			Price:       order.Price,
			Quantity:    1,
			CreatedAt:   time.Now().UTC(),
		},
		{
			ID:          duplicateTradeID,
			Symbol:      initialization.Symbol,
			BuyOrderID:  order.ID,
			SellOrderID: "sell-2",
			Price:       order.Price,
			Quantity:    1,
			CreatedAt:   time.Now().UTC(),
		},
	}

	err = store.PersistSubmission(
		ctx,
		finalOrder,
		nil,
		trades,
		nil,
	)
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	var storedRemainingQuantity int64
	var storedStatus domain.OrderStatus

	err = store.pool.QueryRow(
		ctx,
		`SELECT remaining_quantity, status FROM orders WHERE id = $1`,
		order.ID,
	).Scan(&storedRemainingQuantity, &storedStatus)
	if err != nil {
		t.Fatalf("query order state: %v", err)
	}

	if storedRemainingQuantity != order.RemainingQuantity {
		t.Fatalf("expected remaining quantity %d after rollback, got %d", order.RemainingQuantity, storedRemainingQuantity)
	}

	if storedStatus != order.Status {
		t.Fatalf("expected status %s after rollback, got %s", order.Status, storedStatus)
	}

	var tradeCount int

	err = store.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM trades WHERE id = $1`,
		duplicateTradeID,
	).Scan(&tradeCount)
	if err != nil {
		t.Fatalf("query trade count: %v", err)
	}

	if tradeCount != 0 {
		t.Fatalf("expected trade rollback, found %d rows", tradeCount)
	}
}
