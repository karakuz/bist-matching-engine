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

	now := time.Now().UTC()

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

func TestEngineForBuyOrders(t *testing.T) {
	symbol := createSymbol(t)

	t.Run("price is less than best price", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 299, 100)
		returnedOrder, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}
		if returnedOrder != &order {
			t.Fatalf("expected returned order to be same with order")
		}
		if returnedOrder.Status != domain.StatusOpen{
			t.Fatalf("expected returned to have status OPEN, got: %s", returnedOrder.Status)
		}

		if len(trade) != 0 {
			t.Fatalf("expected trade slice to have length: 0, got %d", len(trade))
		}

		orders := engine.book.GetLevel(domain.SideBuy, 299)
		if orders == nil {
			t.Fatalf("expected level with 299 not nil")
		}
		if orders[0].ID != order.ID {
			t.Fatalf("expected resting order ID %s, got %s", order.ID, orders[0].ID)
		}
	})

	t.Run("best sell does not exists", func(t *testing.T) {
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
		if returnedOrder.Status != domain.StatusOpen{
			t.Fatalf("expected returned to have status OPEN, got: %s", returnedOrder.Status)
		}

		bestBuyOrder, exists := engine.book.BestBuy()
		if !exists {
			t.Fatal("expected incoming buy to rest in book")
		}
		if bestBuyOrder.ID != order.ID {
			t.Fatal("expected incoming buy to be best buy")
		}
	})

	t.Run("full match with best sell order price and quantity", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 302, 100)
		initialBestSellOrder, _ := engine.book.BestSell()

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
		if firstTrade.Quantity != 100 {
			t.Fatalf("expected trade qty to be 100, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID {
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.SellOrderID != initialBestSellOrder.ID {
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302 {
			t.Fatalf("expected firstTrade.Price equal to 302,got: %d", firstTrade.Price)
		}

		if engine.book.GetLastTradePrice() != 302 {
			t.Fatalf("expected last trade price equal to 302, got: %d", engine.book.GetLastTradePrice())
		}
	})

	t.Run("partial fill of incoming order with best sell order", func(t *testing.T) {
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
		if firstTrade.Quantity != 100 {
			t.Fatalf("expected trade qty to be 100, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID {
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302 {
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}

		if engine.book.GetLastTradePrice() != 302 {
			t.Fatalf("expected last trade price equal to 302, got: %d", engine.book.GetLastTradePrice())
		}
	})

	t.Run("partial fill of resting(best sell) order", func(t *testing.T) {
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
		if firstTrade.Quantity != 50 {
			t.Fatalf("expected trade qty to be 50, got %d", firstTrade.Quantity)
		}
		if firstTrade.BuyOrderID != order.ID {
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302 {
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}

		if engine.book.GetLastTradePrice() != 302 {
			t.Fatalf("expected last trade price equal to 302, got: %d", engine.book.GetLastTradePrice())
		}
	})

	t.Run("price exceeds best sell price and best price quantity", func(t *testing.T) {
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
		if firstTrade.BuyOrderID != order.ID {
			t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
		}
		if firstTrade.Price != 302 {
			t.Fatalf("expected firstTrade.Price equal to 302, got: %v", firstTrade.Price)
		}
		if firstTrade.SellOrderID != initialBestSellOrder.ID {
			t.Fatalf("expected firstTrade.SellOrderID equal to initialBestSellOrder.ID, firstTrade.SellOrderID: %v, initialBestSellOrder.ID: %v", firstTrade.SellOrderID, initialBestSellOrder.ID)
		}

		if secondTrade.Quantity != 50 {
			t.Fatalf("expected secondTrade trade qty to be 50, got %d", secondTrade.Quantity)
		}
		if secondTrade.Price != 303 {
			t.Fatalf("expected secondTrade.Price equal to 303, got: %d", secondTrade.Price)
		}
		if secondTrade.SellOrderID != bestSellOrder.ID {
			t.Fatalf("expected secondTrade.SellOrderID equal to bestSellOrder.ID, secondTrade.SellOrderID: %v, initialBestSellOrder.ID: %v", secondTrade.SellOrderID, initialBestSellOrder.ID)
		}

		if engine.book.GetLastTradePrice() != 303 {
			t.Fatalf("expected last trade price equal to 303, got: %d", engine.book.GetLastTradePrice())
		}
	})

	t.Run("price and quantity exceeds all sell orders", func(t *testing.T) {
		tests := []struct {
			name                  string
			buyQty                int64
			wantTradeCount        int64
			wantFirstTradeQty     int64
			wantFirstTradePrice   int64
			wantSecondTradeQty    int64
			wantSecondTradePrice  int64
			wantRemainingQuantity int64
			wantStatus            domain.OrderStatus
			wantBestSellOrder     bool
			wantBestBuyOrderPrice int64
			wantLastTradePrice    int64
		}{
			{
				name:                  "buy quantity exceeds both resting sells",
				buyQty:                75,
				wantTradeCount:        1,
				wantFirstTradeQty:     75,
				wantFirstTradePrice:   302,
				wantSecondTradeQty:    0,
				wantSecondTradePrice:  0,
				wantRemainingQuantity: 0,
				wantStatus:            domain.StatusFilled,
				wantBestSellOrder:     true,
				wantBestBuyOrderPrice: 301,
				wantLastTradePrice:    302,
			}, {
				name:                  "buy quantity matches fully with best sell order",
				buyQty:                100,
				wantTradeCount:        1,
				wantFirstTradeQty:     100,
				wantFirstTradePrice:   302,
				wantSecondTradeQty:    0,
				wantSecondTradePrice:  0,
				wantRemainingQuantity: 0,
				wantStatus:            domain.StatusFilled,
				wantBestSellOrder:     true,
				wantBestBuyOrderPrice: 301,
				wantLastTradePrice:    302,
			}, {
				name:                  "buy quantity matches fully with best sell order, 2nd best resting sell order remains",
				buyQty:                150,
				wantTradeCount:        2,
				wantFirstTradeQty:     100,
				wantFirstTradePrice:   302,
				wantSecondTradeQty:    50,
				wantSecondTradePrice:  303,
				wantRemainingQuantity: 0,
				wantStatus:            domain.StatusFilled,
				wantBestSellOrder:     true,
				wantBestBuyOrderPrice: 301,
				wantLastTradePrice:    303,
			}, {
				name:                  "buy quantity matches fully with 1st and 2nd best sell order",
				buyQty:                200,
				wantTradeCount:        2,
				wantFirstTradeQty:     100,
				wantFirstTradePrice:   302,
				wantSecondTradeQty:    100,
				wantSecondTradePrice:  303,
				wantRemainingQuantity: 0,
				wantStatus:            domain.StatusFilled,
				wantBestSellOrder:     false,
				wantBestBuyOrderPrice: 301,
				wantLastTradePrice:    303,
			}, {
				name:                  "buy quantity matches fully with 1st and 2nd best sell order - 50 incoming qty remains",
				buyQty:                250,
				wantTradeCount:        2,
				wantFirstTradeQty:     100,
				wantFirstTradePrice:   302,
				wantSecondTradeQty:    100,
				wantSecondTradePrice:  303,
				wantRemainingQuantity: 50,
				wantStatus:            domain.StatusPartiallyFilled,
				wantBestSellOrder:     false,
				wantBestBuyOrderPrice: 304,
				wantLastTradePrice:    303,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				orderBook := getTestBook(t)
				engine := NewEngine(&orderBook)
				order := createOrder(t, symbol, domain.SideBuy, 304, tt.buyQty)

				initialBestSellOrder, _ := engine.book.BestSell()

				returnedOrder, trades, err := engine.Submit(&order)
				if err != nil {
					t.Fatalf("engine.Submit failed: %v", err)
				}

				if returnedOrder.RemainingQuantity != tt.wantRemainingQuantity {
					t.Fatalf("expected RemainingQuantity=%d, got %d", tt.wantRemainingQuantity, returnedOrder.RemainingQuantity)
				}
				if returnedOrder.Status != tt.wantStatus {
					t.Fatalf("expected order status = %v, got %v", tt.wantStatus, order.Status)
				}

				_, bestSellOrderExists := engine.book.BestSell()
				if bestSellOrderExists != tt.wantBestSellOrder {
					t.Fatalf("best sell order existence, expected %t, got %t", tt.wantBestSellOrder, bestSellOrderExists)
				}

				bestBuyOrder, bestBuyOrderExists := engine.book.BestBuy()
				if !bestBuyOrderExists {
					t.Fatalf("expected best buy order to be existed, got %t", bestBuyOrderExists)
				}
				if bestBuyOrder.Price != tt.wantBestBuyOrderPrice {
					t.Fatalf("expected best buy order's price %d, got %d", tt.wantBestBuyOrderPrice, bestBuyOrder.Price)
				}
				if returnedOrder.RemainingQuantity != tt.wantRemainingQuantity {
					t.Fatalf("expected returnedOrder.RemainingQuantity to be %d, got: %d", tt.wantRemainingQuantity, returnedOrder.RemainingQuantity)
				}

				if len(trades) != int(tt.wantTradeCount) {
					t.Fatalf("expected %d trades, got %d", tt.wantTradeCount, len(trades))
				}

				firstTrade := trades[0]

				if firstTrade.Quantity != tt.wantFirstTradeQty {
					t.Fatalf("expected first trades qty to be %d, got %d", tt.wantFirstTradeQty, firstTrade.Quantity)
				}
				if firstTrade.BuyOrderID != order.ID {
					t.Fatalf("expected firstTrade.BuyOrderID equal to order.ID, expected: %v, got: %v", order.ID, firstTrade.BuyOrderID)
				}
				if firstTrade.Price != tt.wantFirstTradePrice {
					t.Fatalf("expected firstTrade.Price equal to %d, got: %d", tt.wantFirstTradePrice, firstTrade.Price)
				}
				if firstTrade.SellOrderID != initialBestSellOrder.ID {
					t.Fatalf("expected firstTrade.SellOrderID equal to initialBestSellOrder.ID, firstTrade.SellOrderID: %v, initialBestSellOrder.ID: %v", firstTrade.SellOrderID, initialBestSellOrder.ID)
				}

				if len(trades) == 2 {
					secondTrade := trades[1]

					if secondTrade.Quantity != tt.wantSecondTradeQty {
						t.Fatalf("expected secondTrade trade qty to be %d, got %d", tt.wantSecondTradeQty, secondTrade.Quantity)
					}
					if secondTrade.Price != tt.wantSecondTradePrice {
						t.Fatalf("expected secondTrade.Price equal to %d, got: %d", tt.wantSecondTradePrice, secondTrade.Price)
					}
				}

				if engine.book.GetLastTradePrice() != tt.wantLastTradePrice {
					t.Fatalf("expected last trade price equal to %d, got: %d", engine.book.GetLastTradePrice(), tt.wantLastTradePrice)
				}
			})
		}

	})

	t.Run("time priority at same sell price", func(t *testing.T) {
		tests := []struct {
			name                  string
			buyQty                int64
			wantTradeCount        int
			wantRemainingQuantity int64
		}{
			{
				name:                  "buy quantity exceeds both resting sells",
				buyQty:                25,
				wantTradeCount:        2,
				wantRemainingQuantity: 5,
			},
			{
				name:                  "buy quantity exactly fills both resting sells(multiple fills at same price level)",
				buyQty:                20,
				wantTradeCount:        2,
				wantRemainingQuantity: 0,
			},
			{
				name:                  "buy quantity fills first and partially fills second",
				buyQty:                15,
				wantTradeCount:        2,
				wantRemainingQuantity: 0,
			},
			{
				name:                  "buy quantity only partially fills first",
				buyQty:                5,
				wantTradeCount:        1,
				wantRemainingQuantity: 0,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				orderBook := book.NewBook(symbol)

				t1 := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
				t2 := time.Date(2026, 5, 2, 10, 0, 0, 1, time.UTC)
				t3 := time.Date(2026, 5, 2, 10, 0, 0, 2, time.UTC)

				t1SellOrder, err := domain.NewOrder(uuid.NewString(), symbol, domain.SideSell, 300, 10, t1)
				if err != nil {
					t.Fatalf("domain.NewOrder failed: %v", err)
				}

				t2SellOrder, err := domain.NewOrder(uuid.NewString(), symbol, domain.SideSell, 300, 10, t2)
				if err != nil {
					t.Fatalf("domain.NewOrder failed: %v", err)
				}

				t3BuyOrder, err := domain.NewOrder(uuid.NewString(), symbol, domain.SideBuy, 300, tt.buyQty, t3)
				if err != nil {
					t.Fatalf("domain.NewOrder failed: %v", err)
				}

				if err := orderBook.Add(t1SellOrder, t2SellOrder); err != nil {
					t.Fatalf("orderBook.Add failed: %v", err)
				}

				engine := NewEngine(orderBook)

				returnedOrder, trades, err := engine.Submit(&t3BuyOrder)
				if err != nil {
					t.Fatalf("engine.Submit failed: %v", err)
				}

				if len(trades) != tt.wantTradeCount {
					t.Fatalf("expected %d trades, got %d", tt.wantTradeCount, len(trades))
				}

				if returnedOrder.RemainingQuantity != tt.wantRemainingQuantity {
					t.Fatalf("expected remaining quantity %d, got %d", tt.wantRemainingQuantity, returnedOrder.RemainingQuantity)
				}

				if len(trades) >= 1 && trades[0].SellOrderID != t1SellOrder.ID {
					t.Fatalf("expected first trade to use first resting sell")
				}

				if len(trades) >= 2 && trades[1].SellOrderID != t2SellOrder.ID {
					t.Fatalf("expected second trade to use second resting sell")
				}
			})
		}
	})

	//incoming BUY 305, resting SELL 302, trade price must be 302.
	t.Run("trade price uses resting order price", func(t *testing.T) {
		orderBook := getTestBook(t)
		engine := NewEngine(&orderBook)

		order := createOrder(t, symbol, domain.SideBuy, 305, 100)
		_, trade, err := engine.Submit(&order)

		if err != nil {
			t.Fatalf("engine.Submit failed: %v", err)
		}
		if len(trade) != 1 {
			t.Fatalf("expected 1 trade, got: %d", len(trade))
		}
		if trade[0].Price != 302 {
			t.Fatalf("expected trade price to be 302, got: %d", trade[0].Price)
		}
		if engine.book.GetLastTradePrice() != 302 {
			t.Fatalf("expected last trade price to be 302, got: %d", engine.book.GetLastTradePrice())
		}
	})
}

func TestEngineForSell(t *testing.T) {
	t.Run("", func(t *testing.T) {
		t.Skip("TODO")
	})

}
