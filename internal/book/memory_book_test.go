package book

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
)

var testParticipantId int64 = 1

func createOrder(t *testing.T, symbol Symbol, side domain.Side, price int64, qty int64) domain.Order {
	t.Helper()

	now := time.Now().UTC()

	order, err := domain.NewOrder(
		uuid.NewString(),
		testParticipantId,
		symbol,
		time.Now().UTC(),
		side,
		price,
		qty,
		now)
	if err != nil {
		t.Fatalf("createOrder failed: %v", err)
	}

	return order
}

func TestMemoryBook(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("Error creating symbol: %v", err)
	}

	t.Run("same existing level can't be added to other side", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1050)

		order1 := createOrder(t, symbol, domain.SideBuy, 1050, 100)
		order2 := createOrder(t, symbol, domain.SideSell, 1050, 100)

		addErr := orderBook.Add(order1, order2)
		if addErr != ErrInvalidAddSideConflict {
			t.Fatalf("Expected ErrInvalidAddSideConflict, got %v", addErr)
		}
	})

	t.Run("best buy returns highest price", func(t *testing.T) {
		var smallerPrice int64 = 1050
		var higherPrice int64 = 1100

		orderBook := NewBook(symbol, time.Now().UTC(), 1050)

		order1 := createOrder(t, symbol, domain.SideBuy, smallerPrice, 100)
		order2 := createOrder(t, symbol, domain.SideBuy, higherPrice, 100)

		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
		}

		if err := orderBook.Add(order2); err != nil {
			t.Fatalf("add order2 failed: %v", err)
		}

		bestBuyOrder, exists := orderBook.BestBuy()
		if !exists {
			t.Fatalf("Best Price order does not exists")
		}
		if bestBuyOrder.Price != higherPrice {
			t.Fatalf("unexpected Best Price: %d", bestBuyOrder.Price)
		}
	})

	t.Run("best sell returns lowest price", func(t *testing.T) {
		var smallerPrice int64 = 1050
		var higherPrice int64 = 1100

		orderBook := NewBook(symbol, time.Now().UTC(), 1050)

		order1 := createOrder(t, symbol, domain.SideSell, smallerPrice, 100)
		order2 := createOrder(t, symbol, domain.SideSell, higherPrice, 100)

		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
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
		orderBook := NewBook(symbol, time.Now().UTC(), orderPrice)

		order1 := createOrder(t, symbol, domain.SideBuy, orderPrice, 100)
		order2 := createOrder(t, symbol, domain.SideBuy, orderPrice, 100)

		if err := orderBook.Add(order1); err != nil {
			t.Fatalf("add order1 failed: %v", err)
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

func TestMemoryBookSnapshot(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	orderBook := NewBook(symbol, time.Now().UTC(), 35000)

	levels := []struct {
		side     domain.Side
		price    int64
		quantity int64
	}{
		{side: domain.SideBuy, price: 34990, quantity: 25},
		{side: domain.SideBuy, price: 34990, quantity: 30},
		{side: domain.SideBuy, price: 34980, quantity: 15},
		{side: domain.SideBuy, price: 34970, quantity: 35},
		/* {side: domain.SideBuy, price: 34960, quantity: 18},
		{side: domain.SideBuy, price: 34950, quantity: 1},
		{side: domain.SideBuy, price: 34940, quantity: 7},
		{side: domain.SideBuy, price: 34930, quantity: 74}, */

		{side: domain.SideSell, price: 35000, quantity: 15},
		{side: domain.SideSell, price: 35000, quantity: 25},
		{side: domain.SideSell, price: 35010, quantity: 29},
		{side: domain.SideSell, price: 35020, quantity: 11},
		{side: domain.SideSell, price: 35060, quantity: 14},
		{side: domain.SideSell, price: 35070, quantity: 54},
		{side: domain.SideSell, price: 35090, quantity: 54},
	}

	for _, level := range levels {
		order := createOrder(t, symbol, level.side, level.price, level.quantity)
		orderBook.Add(order)
	}

	bookSnapshot, err := orderBook.Snapshot(5)

	if err != nil {
		t.Fatalf("Error occured on orderBook.Snapshot: %v", err)
	}

	if len(bookSnapshot.Buy) != 3 {
		t.Fatalf("Expected len(bookSnapshot.Buy) to be 3, got %d", len(bookSnapshot.Buy))
	}
	if len(bookSnapshot.Sell) != 5 {
		t.Fatalf("Expected len(bookSnapshot.Sell) to be 5, got %d", len(bookSnapshot.Sell))
	}

	if bookSnapshot.Buy[0].Price != 34990 {
		t.Fatalf("Expected first level of bookSnapshot.Buy's price to be 34990, got %d", bookSnapshot.Buy[0].Price)
	}
	if bookSnapshot.Sell[0].Price != 35000 {
		t.Fatalf("Expected first level of bookSnapshot.Sell's price to be 35000, got %d", bookSnapshot.Sell[0].Price)
	}

	if bookSnapshot.Buy[0].Quantity != 55 {
		t.Fatalf("Expected first level of bookSnapshot.Buy's Quantity to be 55, got %d", bookSnapshot.Buy[0].Quantity)
	}
	if bookSnapshot.Sell[0].Quantity != 40 {
		t.Fatalf("Expected first level of bookSnapshot.Sell's Quantity to be 40, got %d", bookSnapshot.Sell[0].Quantity)
	}
}

func TestMemoryBookSnapshot_EmptyBookReturnsNonNilEmptySlices(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	orderBook := NewBook(symbol, time.Now().UTC(), 35000)
	snapshot, err := orderBook.Snapshot(5)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if snapshot.Buy == nil {
		t.Fatal("expected non-nil empty buy levels")
	}
	if len(snapshot.Buy) != 0 {
		t.Fatalf("expected no buy levels, got %d", len(snapshot.Buy))
	}
	if snapshot.Sell == nil {
		t.Fatal("expected non-nil empty sell levels")
	}
	if len(snapshot.Sell) != 0 {
		t.Fatalf("expected no sell levels, got %d", len(snapshot.Sell))
	}
}

func TestMemoryBookSnapshot_AggregatesRemainingQuantity(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	orderBook := NewBook(symbol, time.Now().UTC(), 35000)
	firstOrder := createOrder(t, symbol, domain.SideBuy, 34990, 100)
	firstOrder.RemainingQuantity = 40
	firstOrder.Status = domain.StatusPartiallyFilled

	secondOrder := createOrder(t, symbol, domain.SideBuy, 34990, 80)
	secondOrder.RemainingQuantity = 25
	secondOrder.Status = domain.StatusPartiallyFilled

	if err := orderBook.Add(firstOrder, secondOrder); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	snapshot, err := orderBook.Snapshot(1)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if len(snapshot.Buy) != 1 {
		t.Fatalf("expected one buy level, got %d", len(snapshot.Buy))
	}
	if snapshot.Buy[0].Quantity != 65 {
		t.Fatalf("expected aggregated remaining quantity 65, got %d", snapshot.Buy[0].Quantity)
	}
}

func TestMemoryBookSnapshot_RejectsNonPositiveLevels(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	orderBook := NewBook(symbol, time.Now().UTC(), 35000)

	for _, levels := range []int64{0, -1, -10} {
		t.Run(fmt.Sprintf("levels_%d", levels), func(t *testing.T) {
			_, err := orderBook.Snapshot(levels)
			if !errors.Is(err, ErrSnapshotSizeNonPositive) {
				t.Fatalf("expected ErrSnapshotSizeNonPositive, got %v", err)
			}
		})
	}
}

func TestMemoryBookMatchCandidates(t *testing.T) {
	symbol, err := domain.NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	t.Run("BUY returns only required SELL orders in price-time order", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1000)

		firstAt1000 := createOrder(t, symbol, domain.SideSell, 1000, 40)
		at990 := createOrder(t, symbol, domain.SideSell, 990, 30)
		secondAt1000 := createOrder(t, symbol, domain.SideSell, 1000, 50)
		at1010 := createOrder(t, symbol, domain.SideSell, 1010, 60)
		aboveLimit := createOrder(t, symbol, domain.SideSell, 1020, 70)

		if err := orderBook.Add(
			firstAt1000,
			at990,
			secondAt1000,
			at1010,
			aboveLimit,
		); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		incoming := createOrder(t, symbol, domain.SideBuy, 1010, 120)

		candidates, err := orderBook.MatchCandidates(incoming)
		if err != nil {
			t.Fatalf("MatchCandidates failed: %v", err)
		}

		wantIDs := []string{
			at990.ID,
			firstAt1000.ID,
			secondAt1000.ID,
		}

		if len(candidates) != len(wantIDs) {
			t.Fatalf(
				"expected %d candidates, got %d",
				len(wantIDs),
				len(candidates),
			)
		}

		for index, wantID := range wantIDs {
			if candidates[index].ID != wantID {
				t.Fatalf(
					"candidate %d: expected order ID %s, got %s",
					index,
					wantID,
					candidates[index].ID,
				)
			}
		}
	})

	t.Run("SELL returns only required BUY orders in price-time order", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1000)

		firstAt1000 := createOrder(t, symbol, domain.SideBuy, 1000, 40)
		at1010 := createOrder(t, symbol, domain.SideBuy, 1010, 30)
		secondAt1000 := createOrder(t, symbol, domain.SideBuy, 1000, 50)
		belowLimit := createOrder(t, symbol, domain.SideBuy, 990, 60)

		if err := orderBook.Add(
			firstAt1000,
			at1010,
			secondAt1000,
			belowLimit,
		); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		incoming := createOrder(t, symbol, domain.SideSell, 1000, 100)

		candidates, err := orderBook.MatchCandidates(incoming)
		if err != nil {
			t.Fatalf("MatchCandidates failed: %v", err)
		}

		wantIDs := []string{
			at1010.ID,
			firstAt1000.ID,
			secondAt1000.ID,
		}

		if len(candidates) != len(wantIDs) {
			t.Fatalf(
				"expected %d candidates, got %d",
				len(wantIDs),
				len(candidates),
			)
		}

		for index, wantID := range wantIDs {
			if candidates[index].ID != wantID {
				t.Fatalf(
					"candidate %d: expected order ID %s, got %s",
					index,
					wantID,
					candidates[index].ID,
				)
			}
		}
	})

	t.Run("skips exhausted resting orders", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1000)

		exhausted := createOrder(t, symbol, domain.SideSell, 1000, 20)
		exhausted.RemainingQuantity = 0
		exhausted.Status = domain.StatusFilled

		open := createOrder(t, symbol, domain.SideSell, 1000, 20)

		if err := orderBook.Add(exhausted, open); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		incoming := createOrder(t, symbol, domain.SideBuy, 1000, 10)

		candidates, err := orderBook.MatchCandidates(incoming)
		if err != nil {
			t.Fatalf("MatchCandidates failed: %v", err)
		}

		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}
		if candidates[0].ID != open.ID {
			t.Fatalf(
				"expected open order ID %s, got %s",
				open.ID,
				candidates[0].ID,
			)
		}
	})

	t.Run("returns order copies without mutating the book", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1000)
		resting := createOrder(t, symbol, domain.SideSell, 1000, 50)

		if err := orderBook.Add(resting); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		incoming := createOrder(t, symbol, domain.SideBuy, 1000, 10)

		candidates, err := orderBook.MatchCandidates(incoming)
		if err != nil {
			t.Fatalf("MatchCandidates failed: %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("expected 1 candidate, got %d", len(candidates))
		}

		candidates[0].RemainingQuantity = 0
		candidates[0].Status = domain.StatusFilled

		stored := orderBook.sells[resting.Price][0]
		if stored.RemainingQuantity != resting.RemainingQuantity {
			t.Fatalf(
				"expected stored remaining quantity %d, got %d",
				resting.RemainingQuantity,
				stored.RemainingQuantity,
			)
		}
		if stored.Status != resting.Status {
			t.Fatalf(
				"expected stored status %s, got %s",
				resting.Status,
				stored.Status,
			)
		}
	})

	t.Run("rejects invalid incoming side", func(t *testing.T) {
		orderBook := NewBook(symbol, time.Now().UTC(), 1000)
		incoming := createOrder(t, symbol, domain.SideBuy, 1000, 10)
		incoming.Side = domain.Side("HOLD")

		_, err := orderBook.MatchCandidates(incoming)
		if !errors.Is(err, domain.ErrInvalidSide) {
			t.Fatalf(
				"expected ErrInvalidSide, got %v",
				err,
			)
		}
	})
}
