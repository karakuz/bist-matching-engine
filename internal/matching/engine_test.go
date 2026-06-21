package matching

import (
	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
)

func getTestBook(t *testing.T) *book.Book {
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

	order1, err := domain.NewOrder(uuid.NewString(), "ASELS", domain.SideBuy, 300 /*price*/, 100 /*qty*/, now)
	if err != nil {
		t.Fatalf("getTestBook order1 failed: %v", err)
	}

	now2 := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	order2, err := domain.NewOrder(uuid.NewString(), "ASELS", domain.SideBuy, 301 /*price*/, 100 /*qty*/, now2)
	if err != nil {
		t.Fatalf("getTestBook order2 failed: %v", err)
	}

	now3 := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	order3, err := domain.NewOrder(uuid.NewString(), "ASELS", domain.SideSell, 302 /*price*/, 100 /*qty*/, now3)
	if err != nil {
		t.Fatalf("getTestBook order3 failed: %v", err)
	}

	now4 := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
	order4, err := domain.NewOrder(uuid.NewString(), "ASELS", domain.SideSell, 303 /*price*/, 100 /*qty*/, now4)
	if err != nil {
		t.Fatalf("getTestBook order4 failed: %v", err)
	}

	orderBook := book.NewBook()
	orderBook.Add(order1)
	orderBook.Add(order2)
	orderBook.Add(order3)
	orderBook.Add(order4)

	return orderBook
}

func TestEngine(t *testing.T) {
	t.Run("empty book creates no trade", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("side buy - full match", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(orderBook)

		now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
		order, err := domain.NewOrder(uuid.NewString(), "ASELS", domain.SideBuy, 302 /*price*/, 100 /*qty*/, now)
		if err != nil {
			t.Fatalf("getTestBook order failed: %v", err)
		}

		returnedOrder, trade, err := engine.Submit(&order)

		if returnedOrder.RemainingQuantity != 0 {
			t.Fatalf("expected RemainingQuantity=0, got %d", returnedOrder.RemainingQuantity)
		}
		if order.Status != domain.StatusFilled {
			t.Fatalf("expected order status = StatusFilled, got %v", order.Status)
		}

		bestSellOrder, bestSellOrderExists := engine.book.BestSell()
		if !bestSellOrderExists {
			t.Fatalf("expected best sell order to be expected, got %v", bestSellOrderExists)
		}
		if bestSellOrder.Price != 303 {
			t.Fatalf("expected best sell order to have 303 price, got %d", bestSellOrder.Price)
		}
		if bestSellOrder.RemainingQuantity != 100 {
			t.Fatalf("expected best sell order's remaining qty to be 100, got %d", bestSellOrder.RemainingQuantity)
		}

		if len(trade) != 1 {
			t.Fatalf("expected trade slice to have length: 1, got %d", len(trade))
		}

		t.Skip("TODO")
	})

	t.Run("no match because of price", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("partial fill of incoming order", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("partial fill of resting order", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("multiple fills", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("price priority", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("time priority", func(t *testing.T) {
		t.Skip("TODO")
	})

	//BUY 10.10 | SELL 10.00
	t.Run("trade price uses resting order price", func(t *testing.T) {
		t.Skip("TODO")
	})

}
