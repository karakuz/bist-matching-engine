package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresStore_GetSeededRestingOrdersForSession(t *testing.T) {
	store, ctx := newTestStore(t)

	initializations, err := store.GetBookInitializations(ctx)
	if err != nil {
		t.Fatalf("GetBookInitializations failed: %v", err)
	}

	var aselsInitialization BookInitialization
	for _, initialization := range initializations {
		if initialization.Symbol.Code == "ASELS" {
			aselsInitialization = initialization
			break
		}
	}
	if aselsInitialization.Symbol.Code == "" {
		t.Fatal("ASELS book initialization not found")
	}

	orders, err := store.GetRestingOrdersForSession(
		ctx,
		aselsInitialization.Symbol,
		aselsInitialization.SessionDate,
	)
	if err != nil {
		t.Fatalf("GetRestingOrdersForSession failed: %v", err)
	}

	seedPrefix := "seed-asels-" + aselsInitialization.SessionDate.Format("20060102")
	seedOrderCount := 0
	bestBid := int64(0)
	bestAsk := int64(0)
	type priceLevel struct {
		side  domain.Side
		price int64
	}
	ordersBySideAndPrice := make(map[priceLevel]int)

	for _, order := range orders {
		if !strings.HasPrefix(order.ID, seedPrefix) {
			continue
		}

		seedOrderCount++
		levelKey := priceLevel{side: order.Side, price: order.Price}
		ordersBySideAndPrice[levelKey]++

		if order.Side == domain.SideBuy && order.Price > bestBid {
			bestBid = order.Price
		}
		if order.Side == domain.SideSell && (bestAsk == 0 || order.Price < bestAsk) {
			bestAsk = order.Price
		}
	}

	if seedOrderCount != 52 {
		t.Fatalf("expected 52 seeded ASELS orders, got %d", seedOrderCount)
	}
	if bestBid >= bestAsk {
		t.Fatalf("seeded orders cross: best bid %d, best ask %d", bestBid, bestAsk)
	}

	hasMultipleOrdersAtLevel := false
	for _, count := range ordersBySideAndPrice {
		if count > 1 {
			hasMultipleOrdersAtLevel = true
			break
		}
	}
	if !hasMultipleOrdersAtLevel {
		t.Fatal("expected multiple seeded orders at the same price level")
	}
}

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

	var testParticipantId int64 = 1

	initialization, err := store.GetBookInitialization(ctx, "ASELS")
	if err != nil {
		t.Fatalf("store.GetBookInitialization failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		testParticipantId,
		initialization.Symbol,
		initialization.SessionDate,
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

	got, err := store.GetOrderByID(ctx, order.ID, initialization.Symbol)
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

	var testParticipantId int64 = 1

	initialization, err := store.GetBookInitialization(ctx, "ASELS")
	if err != nil {
		t.Fatalf("store.GetBookInitialization failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		testParticipantId,
		initialization.Symbol,
		initialization.SessionDate,
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

	if err := store.UpdateOrders(ctx, []domain.Order{order}); err != nil {
		t.Fatalf("UpdateOrder failed: %v", err)
	}
	t.Cleanup(func() {
		store.pool.Exec(ctx, "DELETE FROM orders WHERE id = $1", order.ID)
	})

	got, err := store.GetOrderByID(ctx, order.ID, initialization.Symbol)
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
