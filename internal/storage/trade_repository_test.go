package storage

import (
	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testParticipantId int64 = 1

func createSymbol(t *testing.T) domain.Symbol {
	t.Helper()

	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("createSymbol failed: %v", err)
	}

	return symbol
}

func createOrder(t *testing.T, symbol domain.Symbol, side domain.Side, price int64, qty int64) domain.Order {
	t.Helper()

	now := time.Now().UTC()

	order, err := domain.NewOrder(
		uuid.NewString(),
		testParticipantId,
		symbol,
		now,
		side,
		price,
		qty,
		now)
	if err != nil {
		t.Fatalf("createOrder failed: %v", err)
	}

	return order
}

func getTestBook(t *testing.T) book.Book {
	t.Helper()

	symbol := createSymbol(t)

	order1 := createOrder(t, symbol, domain.SideBuy, 300, 100)
	order2 := createOrder(t, symbol, domain.SideBuy, 301, 100)
	order3 := createOrder(t, symbol, domain.SideSell, 302, 100)
	order4 := createOrder(t, symbol, domain.SideSell, 303, 100)

	orderBook := book.NewBook(symbol, time.Now().UTC(), 300)
	orderBook.Add(order1)
	orderBook.Add(order2)
	orderBook.Add(order3)
	orderBook.Add(order4)

	return *orderBook
}

func assertTradeEqual(t *testing.T, want domain.Trade, got domain.Trade) {
	t.Helper()

	if got.ID != want.ID {
		t.Fatalf("expected ID %s, got %s", want.ID, got.ID)
	}
	if got.Symbol.Code != want.Symbol.Code {
		t.Fatalf("expected symbol %s, got %s", want.Symbol.Code, got.Symbol.Code)
	}
	if got.BuyOrderID != want.BuyOrderID {
		t.Fatalf("expected buy order ID %s, got %s", want.BuyOrderID, got.BuyOrderID)
	}
	if got.SellOrderID != want.SellOrderID {
		t.Fatalf("expected sell order ID %s, got %s", want.SellOrderID, got.SellOrderID)
	}
	if got.Price != want.Price {
		t.Fatalf("expected price %d, got %d", want.Price, got.Price)
	}
	if got.Quantity != want.Quantity {
		t.Fatalf("expected quantity %d, got %d", want.Quantity, got.Quantity)
	}

	if !got.CreatedAt.UTC().Equal(want.CreatedAt.UTC()) {
		t.Fatalf("expected created_at %v, got %v", want.CreatedAt, got.CreatedAt)
	}
}

func TestPostgresStore_InsertTrade(t *testing.T) {
	store, ctx := newTestStore(t)

	symbol := createSymbol(t)
	orderBook := getTestBook(t)
	engine := matching.NewEngine(&orderBook)

	order := createOrder(t, symbol, domain.SideBuy, 303, 150)

	matchResult, err := engine.Submit(&order)
	if err != nil {
		t.Fatalf("engine.Submit failed: %v", err)
	}

	trades := matchResult.Trades
	err = store.InsertTrades(ctx, trades)
	if err != nil {
		t.Fatalf("InsertTrades failed: %v", err)
	}
	t.Cleanup(func() {
		store.pool.Exec(ctx, "DELETE FROM trades WHERE id = ANY($1)", []string{
			trades[0].ID,
			trades[1].ID,
		})
	})

	firstTrade, err := store.GetTrade(ctx, trades[0].ID)
	if err != nil {
		t.Fatalf("store.GetTrade failed: %v", err)
	}

	secondTrade, err := store.GetTrade(ctx, trades[1].ID)

	assertTradeEqual(t, trades[0], firstTrade)
	assertTradeEqual(t, trades[1], secondTrade)
}
