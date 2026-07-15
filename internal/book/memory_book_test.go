package book

import (
	"bist-matching-engine/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testParticipantId int64 = 1

func TestMemoryBook(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("Error creating symbol: %v", err)
	}

	t.Run("same existing level can't be added to other side", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 300)

		order1, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideBuy,
			1050,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order1 failed: %v", err)
		}

		order2, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideSell,
			1050,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order1 failed: %v", err)
		}

		addErr := orderBook.Add(order1, order2)
		if addErr != ErrInvalidAddSideConflict {
			t.Fatalf("Expected ErrInvalidAddSideConflict, got %v", addErr)
		}
	})

	t.Run("best buy returns highest price", func(t *testing.T) {
		var smallerPrice int64 = 1050
		var highestPrice int64 = 1100

		orderBook := NewBook(symbol, time.Now().UTC(), 300)

		order1, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideBuy,
			smallerPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order1 failed: %v", err)
		}
		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
		}

		order2, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideBuy,
			highestPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order2 failed: %v", err)
		}
		if err := orderBook.Add(order2); err != nil {
			t.Fatalf("add order2 failed: %v", err)
		}

		bestBuyOrder, exists := orderBook.BestBuy()
		if !exists {
			t.Fatalf("Best Price order does not exists")
		}
		if bestBuyOrder.Price != highestPrice {
			t.Fatalf("unexpected Best Price: %d", bestBuyOrder.Price)
		}
	})

	t.Run("best sell returns lowest price", func(t *testing.T) {
		var smallerPrice int64 = 1050
		var highestPrice int64 = 1100

		orderBook := NewBook(symbol, time.Now().UTC(), 300)

		order1, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideSell,
			smallerPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order1 failed: %v", err)
		}
		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
		}

		order2, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideSell,
			highestPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order2 failed: %v", err)
		}
		if err := orderBook.Add(order2); err != nil {
			t.Fatalf("add order2 failed: %v", err)
		}

		bestSellOrder, exists := orderBook.BestSell()
		if !exists {
			t.Fatalf("Best Price order does not exists")
		}
		if bestSellOrder.Price != smallerPrice {
			t.Fatalf("unexpected Best Price: %d", bestSellOrder.Price)
		}
	})

	t.Run("same price preserves FIFO", func(t *testing.T) {
		const orderPrice int64 = 1000
		orderBook := NewBook(symbol, time.Now().UTC(), 300)

		order1, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideBuy,
			orderPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order1 failed: %v", err)
		}
		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
		}

		order2, err := domain.NewOrder(
			uuid.NewString(),
			testParticipantId,
			symbol,
			time.Now().UTC(),
			domain.SideBuy,
			orderPrice,
			100,
			time.Now().UTC())
		if err != nil {
			t.Fatalf("new order2 failed: %v", err)
		}
		if err := orderBook.Add(order2); err != nil {
			t.Fatalf("add order2 failed: %v", err)
		}

		//level = one price bucket.
		level := orderBook.buys[orderPrice]
		if len(level) == 0 {
			t.Fatalf("expected non-empty level")
		}

		firstOrder := level[0]
		if firstOrder.ID != order1.ID {
			t.Fatalf("expected first order id %s, got %s", order1.ID, firstOrder.ID)
		}

		lastOrder := level[len(level)-1]
		if lastOrder.ID != order2.ID {
			t.Fatalf("expected last order id %s, got %s", order2.ID, lastOrder.ID)
		}
	})

	t.Run("empty book returns no order", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 300)
		if len(orderBook.buys) != 0 || len(orderBook.sells) != 0 {
			t.Fatalf("Empty order book has orders, expected no order")
		}

		_, bestBuyExists := orderBook.BestBuy()
		if bestBuyExists {
			t.Fatalf("Empty order book has best buy, expected no best buy")
		}

		_, bestSellExists := orderBook.BestSell()
		if bestSellExists {
			t.Fatalf("Empty order book has best sell, expected no best sell")
		}
	})

}

func TestNewBookStoresSessionDate(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	sessionDate := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	orderBook := NewBook(symbol, sessionDate, 35000)

	if !orderBook.SessionDate.Equal(sessionDate) {
		t.Fatalf("expected session date %v, got %v", sessionDate, orderBook.SessionDate)
	}
}
