package matching

import (
	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
)

type Symbol = domain.Symbol

func createSymbol(t *testing.T) domain.Symbol {
	t.Helper()

	symbol, err := domain.NewSymbol("ASELS", 1)
	if err != nil {
		t.Fatalf("createSymbol failed: %v", err)
	}

	return symbol
}

func createOrder(t *testing.T, symbol Symbol, side domain.Side, price int64, qty int64) domain.Order {
	t.Helper()

	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

	order, err := domain.NewOrder(uuid.NewString(), symbol, side, price, qty, now)
	if err != nil {
		t.Fatalf("createOrder failed: %v", err)
	}

	return order
}

func getTestBook(t *testing.T) book.Book {
	symbol := createSymbol(t)

	order1 := createOrder(t, symbol, domain.SideBuy, 300, 100)
	order2 := createOrder(t, symbol, domain.SideBuy, 301, 100)
	order3 := createOrder(t, symbol, domain.SideSell, 302, 100)
	order4 := createOrder(t, symbol, domain.SideSell, 303, 100)

	orderBook := book.NewBook(symbol)
	orderBook.Add(order1)
	orderBook.Add(order2)
	orderBook.Add(order3)
	orderBook.Add(order4)

	return *orderBook
}

func TestEngine(t *testing.T) {
	symbol := createSymbol(t)

	t.Run("incoming buy order - best sell does not exists", func(t *testing.T) {
		orderBook := book.NewBook(symbol)
		engine := NewEngine(orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 300, 100)

		returnedOrder, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}
		if order != *returnedOrder {
			t.Fatalf("expected returnedOrder = order, got %+v want %+v", *returnedOrder, order)
		}
		if len(trade) != 0 {
			t.Fatalf("expected len(trade) to be 0, got %d", len(trade))
		}
		if returnedOrder.RemainingQuantity != 100 {
			t.Fatalf("expected returnedOrder.RemainingQuantity to be 100, got %d", returnedOrder.RemainingQuantity)
		}		
	})

	t.Run("incoming buy order - full match with best sell order price and quantity", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		symbol := createSymbol(t)
		order := createOrder(t, symbol, domain.SideBuy, 302, 100)
		initialBestSellOrder, _  := engine.book.BestSell()
	
		returnedOrder, trade, err := engine.Submit(&order)
		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}

		if returnedOrder.RemainingQuantity != 0 {
			t.Fatalf("expected RemainingQuantity=0, got %d", returnedOrder.RemainingQuantity)
		}
		if returnedOrder.Status != domain.StatusFilled {
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

		firstTrade := trade[0]
		if firstTrade.Quantity != 100{
			t.Fatalf("expected trade qty to be 100, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID{
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.SellOrderID != initialBestSellOrder.ID{
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302{
			t.Fatalf("expected firstTrade.Price equal to 302,got: %v", firstTrade.Price)
		}
	})

	t.Run("incoming buy order - partial fill of incoming order with best sell order", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 302, 150)

		returnedOrder, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}

		if returnedOrder.RemainingQuantity != 50 {
			t.Fatalf("expected RemainingQuantity=50, got %d", returnedOrder.RemainingQuantity)
		}
		if returnedOrder.Status != domain.StatusPartiallyFilled {
			t.Fatalf("expected order status = StatusPartiallyFilled, got %v", order.Status)
		}

		if len(trade) != 1 {
			t.Fatalf("expected trade slice to have length: 1, got %d", len(trade))
		}

		bestBuyOrder, bestBuyOrderExists := engine.book.BestBuy()
		if !bestBuyOrderExists {
			t.Fatalf("expected best buy order to be expected, got %v", bestBuyOrderExists)
		}
		if bestBuyOrder.Price != 302 {
			t.Fatalf("expected best sell order to have 303 price, got %d", bestBuyOrder.Price)
		}
		if bestBuyOrder.RemainingQuantity != 50 {
			t.Fatalf("expected best buy order's remaining qty to be 100, got %d", bestBuyOrder.RemainingQuantity)
		}

		firstTrade := trade[0]
		if firstTrade.Quantity != 100{
			t.Fatalf("expected trade qty to be 100, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID{
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302{
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}
	})

	t.Run("incoming buy order - partial fill of resting(best sell) order", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 302, 50)

		returnedOrder, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}
		if returnedOrder.RemainingQuantity != 0 {
			t.Fatalf("expected RemainingQuantity=0, got %d", returnedOrder.RemainingQuantity)
		}
		if returnedOrder.Status != domain.StatusFilled {
			t.Fatalf("expected order status = StatusFilled, got %v", order.Status)
		}

		bestSellOrder, bestSellOrderExists := engine.book.BestSell()
		if !bestSellOrderExists {
			t.Fatalf("expected best sell order to be expected, got %v", bestSellOrderExists)
		}
		if bestSellOrder.Price != 302 {
			t.Fatalf("expected best sell order to have 302 price, got %d", bestSellOrder.Price)
		}
		if bestSellOrder.RemainingQuantity != 50 {
			t.Fatalf("expected best sell order's remaining qty to be 50, got %d", bestSellOrder.RemainingQuantity)
		}
		if bestSellOrder.Status != domain.StatusPartiallyFilled {
			t.Fatalf("expected best sell order's status to be 'StatusPartiallyFilled', got %v", bestSellOrder.Status)
		}

		if len(trade) != 1 {
			t.Fatalf("expected trade slice to have length: 1, got %d", len(trade))
		}
		firstTrade := trade[0]
		if firstTrade.Quantity != 50{
			t.Fatalf("expected trade qty to be 50, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID{
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302{
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}
	})

	t.Run("incoming buy order - price exceeds best sell price and best price quantity", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 303, 150)
		initialBestSellOrder, _ := engine.book.BestSell()

		returnedOrder, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}

		if len(trade) != 2 {
			t.Fatalf("expected 2 trades, got %d", len(trade))
		}

		if returnedOrder.RemainingQuantity != 0 {
			t.Fatalf("expected RemainingQuantity=0, got %d", returnedOrder.RemainingQuantity)
		}
		if returnedOrder.Status != domain.StatusFilled {
			t.Fatalf("expected order status = StatusFilled, got %v", order.Status)
		}

		bestSellOrder, bestSellOrderExists := engine.book.BestSell()
		if !bestSellOrderExists {
			t.Fatalf("expected best sell order to be existed, got %v", bestSellOrderExists)
		}
		if bestSellOrder.Price != 303 {
			t.Fatalf("expected best sell order to have 302 price, got %d", bestSellOrder.Price)
		}
		if bestSellOrder.RemainingQuantity != 50 {
			t.Fatalf("expected best sell order's remaining qty to be 50, got %d", bestSellOrder.RemainingQuantity)
		}
		if bestSellOrder.Status != domain.StatusPartiallyFilled {
			t.Fatalf("expected best sell order's status to be 'StatusPartiallyFilled', got %v", bestSellOrder.Status)
		}

		firstTrade := trade[0]
		secondTrade := trade[1]

		if firstTrade.Quantity != 100 {
			t.Fatalf("expected first trade qty to be 100, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID{
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302{
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}
		if firstTrade.SellOrderID != initialBestSellOrder.ID{
			t.Fatalf("expected firstTrade.SellOrderID equal to initialBestSellOrder.ID, firstTrade.SellOrderID: %v, initialBestSellOrder.ID: %v", firstTrade.SellOrderID, initialBestSellOrder.ID)
		}

		if secondTrade.Quantity != 50 {
			t.Fatalf("expected secondTrade trade qty to be 50, got %d", secondTrade.Quantity)
		}
		if secondTrade.Price != 303{
			t.Fatalf("expected secondTrade.Price equal to 303, got: %d", secondTrade.Price)
		}
		if secondTrade.SellOrderID != bestSellOrder.ID{
			t.Fatalf("expected secondTrade.SellOrderID equal to bestSellOrder.ID, secondTrade.SellOrderID: %v, initialBestSellOrder.ID: %v", secondTrade.SellOrderID, initialBestSellOrder.ID)
		}

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
