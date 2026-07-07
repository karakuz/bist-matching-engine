package matching

import (
	"errors"
	//"fmt"
	"time"

	"github.com/google/uuid"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
)

var (
	ErrUnhandledCase = errors.New("Unhandled case error")
)

type Engine struct {
	book *book.Book
}

func NewEngine(orderBook *book.Book) *Engine {
	return &Engine{book: orderBook}
}

func newTrade(engine *Engine, order *domain.Order, restingOrder domain.Order, quantity int64) domain.Trade {
	engine.book.UpdateLastTradePrice(restingOrder.Price)

	var BuyOrderID string
	var SellOrderID string
	if order.Side == domain.SideBuy{
		BuyOrderID = order.ID
		SellOrderID = restingOrder.ID
	} else {
		BuyOrderID = restingOrder.ID
		SellOrderID = order.ID
	}

	return domain.Trade{
		ID:          uuid.NewString(),
		Symbol:      order.Symbol,
		BuyOrderID:  BuyOrderID,
		SellOrderID: SellOrderID,
		Price:       restingOrder.Price,
		Quantity:    quantity,
		CreatedAt:   time.Now().UTC().Truncate(time.Microsecond),
	}
}

func submitBid(engine *Engine, order *domain.Order) (*domain.Order, []domain.Trade, error) {
	var err error = nil
	trades := make([]domain.Trade, 0)
	bestSellOrder, bestSellOrderExists := engine.book.BestSell()

	if !bestSellOrderExists {
		err = engine.book.Add(*order)
		return order, trades, err
	}

	if order.Price < bestSellOrder.Price {
		err = engine.book.Add(*order)
		return order, trades, err
	} else if order.Price == bestSellOrder.Price {
		for bestSellOrderExists && order.Price == bestSellOrder.Price && order.RemainingQuantity > 0 {
			tradeQty := min(order.RemainingQuantity, bestSellOrder.RemainingQuantity)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}
			engine.book.ReduceBestSell(tradeQty)

			trade := newTrade(engine, order, bestSellOrder, tradeQty)
			trades = append(trades, trade)

			bestSellOrder, bestSellOrderExists = engine.book.BestSell()
		}

		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}

		return order, trades, err
	} else if order.Price > bestSellOrder.Price {
		for bestSellOrderExists &&
			order.Price >= bestSellOrder.Price &&
			order.RemainingQuantity != 0 {

			tradeQty := min(bestSellOrder.RemainingQuantity, order.RemainingQuantity)

			trade := newTrade(engine, order, bestSellOrder, tradeQty)
			trades = append(trades, trade)
			engine.book.ReduceBestSell(tradeQty)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}

			//next best sell order
			bestSellOrder, bestSellOrderExists = engine.book.BestSell()
		}

		//if all sell orders >= order.Price are traded and buy order has remaining qty
		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}

		return order, trades, err
	}

	return order, trades, ErrUnhandledCase
}

func submitAsk(engine *Engine, order *domain.Order) (*domain.Order, []domain.Trade, error) {
	var err error = nil
	trades := make([]domain.Trade, 0)
	bestBuyOrder, bestBuyOrderExists := engine.book.BestBuy()

	if !bestBuyOrderExists {
		err = engine.book.Add(*order)
		return order, trades, err
	}

	if order.Price > bestBuyOrder.Price {
		err = engine.book.Add(*order)
		return order, trades, err
	} else if order.Price == bestBuyOrder.Price {
		for bestBuyOrderExists && order.Price <= bestBuyOrder.Price && order.RemainingQuantity > 0 {
			tradeQty := min(order.RemainingQuantity, bestBuyOrder.RemainingQuantity)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}
			engine.book.ReduceBestBuy(tradeQty)

			trade := newTrade(engine, order, bestBuyOrder, tradeQty)
			trades = append(trades, trade)

			bestBuyOrder, bestBuyOrderExists = engine.book.BestBuy()
		}

		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}

		return order, trades, err
	} else if order.Price < bestBuyOrder.Price {
		for bestBuyOrderExists &&
			order.Price <= bestBuyOrder.Price &&
			order.RemainingQuantity != 0 {

			tradeQty := min(bestBuyOrder.RemainingQuantity, order.RemainingQuantity)

			trade := newTrade(engine, order, bestBuyOrder, tradeQty)
			trades = append(trades, trade)
			engine.book.ReduceBestBuy(tradeQty)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}

			//next best buy order
			bestBuyOrder, bestBuyOrderExists = engine.book.BestBuy()
		}

		//if order has remaining qty
		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}
		return order, trades, err
	}

	return order, trades, ErrUnhandledCase
}

func (engine *Engine) Submit(order *domain.Order) (*domain.Order, []domain.Trade, error) {
	if order.Side == domain.SideBuy {
		return submitBid(engine, order)
	} else if order.Side == domain.SideSell {
		return submitAsk(engine, order)
	}

	return order, make([]domain.Trade, 0), ErrUnhandledCase
}
