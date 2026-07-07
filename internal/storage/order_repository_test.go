package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestStore(t *testing.T) (*PostgresStore, context.Context) {
	t.Helper()

	conString := os.Getenv("bme_pg_connstring")
	if conString == "" {
		t.Skip("bme_pg_connstring not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, conString)
	if err != nil {
		t.Fatalf("pgxpool.New failed: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return NewPostgresStore(pool), ctx
}

func TestPostgresStore_InsertAndGetOrderByID(t *testing.T) {
	store, ctx := newTestStore(t)

	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		symbol,
		domain.SideBuy,
		1050,
		100,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	if err := store.InsertOrder(ctx, order); err != nil {
		t.Fatalf("InsertOrder failed: %v", err)
	}

	got, err := store.GetOrderByID(ctx, order.ID, symbol)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}
	t.Cleanup(func() {
		store.pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", order.ID)
	})

	if got.ID != order.ID {
		t.Fatalf("expected ID %s, got %s", order.ID, got.ID)
	}
	if got.Symbol.Code != order.Symbol.Code {
		t.Fatalf("expected symbol %s, got %s", order.Symbol.Code, got.Symbol.Code)
	}
	if got.Side != order.Side {
		t.Fatalf("expected side %s, got %s", order.Side, got.Side)
	}
	if got.Price != order.Price {
		t.Fatalf("expected price %d, got %d", order.Price, got.Price)
	}
	if got.Quantity != order.Quantity {
		t.Fatalf("expected quantity %d, got %d", order.Quantity, got.Quantity)
	}
	if got.RemainingQuantity != order.RemainingQuantity {
		t.Fatalf("expected remaining quantity %d, got %d", order.RemainingQuantity, got.RemainingQuantity)
	}
	if got.Status != order.Status {
		t.Fatalf("expected status %s, got %s", order.Status, got.Status)
	}
}

func TestPostgresStore_UpdateOrder(t *testing.T) {
	store, ctx := newTestStore(t)

	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		symbol,
		domain.SideSell,
		1060,
		100,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	if err := store.InsertOrder(ctx, order); err != nil {
		t.Fatalf("InsertOrder failed: %v", err)
	}

	order.RemainingQuantity = 40
	order.Status = domain.StatusPartiallyFilled

	if err := store.UpdateOrder(ctx, order); err != nil {
		t.Fatalf("UpdateOrder failed: %v", err)
	}
	t.Cleanup(func() {
		store.pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", order.ID)
	})

	got, err := store.GetOrderByID(ctx, order.ID, symbol)
	if err != nil {
		t.Fatalf("GetOrderByID failed: %v", err)
	}

	if got.RemainingQuantity != 40 {
		t.Fatalf("expected remaining quantity 40, got %d", got.RemainingQuantity)
	}
	if got.Status != domain.StatusPartiallyFilled {
		t.Fatalf("expected status %s, got %s", domain.StatusPartiallyFilled, got.Status)
	}
}