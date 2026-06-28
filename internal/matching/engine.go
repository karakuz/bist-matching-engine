package matching

import (
	"errors"
	//"fmt"
	"time"

	"github.com/google/uuid"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
)

// TODO: implement submit + matching logic (price-time priority)
// TODO: create trades using resting order price

var (
	ErrUnhandledCase = errors.New("Unhandled case error")
)

type Engine struct {
	book *book.Book
}

func NewEngine(orderBook *book.Book) *Engine {
	return &Engine{book: orderBook}
}

func newTrade(engine *Engine, order *domain.Order, restingOrder domain.Order, quantity int64) domain.Trade{
	engine.book.UpdateLastTradePrice(restingOrder.Price)

	return domain.Trade{
		ID:          uuid.NewString(),
		Symbol:      order.Symbol,
		BuyOrderID:  order.ID,
		SellOrderID: restingOrder.ID,
		Price:       restingOrder.Price,
		Quantity:    quantity,
		CreatedAt:   time.Now().UTC(),
	}
}

func submitBid(engine *Engine, order *domain.Order) (*domain.Order, []domain.Trade, error){
	trades := make([]domain.Trade, 0)
	bestSellOrder, bestSellOrderExists := engine.book.BestSell()

	if !bestSellOrderExists {
		engine.book.Add(*order)
		return order, trades, nil
	}

	if order.Price < bestSellOrder.Price {
		engine.book.Add(*order)
		return order, trades, nil
	} else if order.Price == bestSellOrder.Price {
		if bestSellOrder.RemainingQuantity >= order.RemainingQuantity {
			//incoming order full match
			trade := newTrade(engine, order, bestSellOrder, order.RemainingQuantity)
			trades = append(trades, trade)

			engine.book.ReduceBestSell(order.RemainingQuantity)

			order.RemainingQuantity = 0
			order.Status = domain.StatusFilled

			return order, trades, nil
		} else if bestSellOrder.RemainingQuantity < order.RemainingQuantity {
			//resting order full match
			trade := newTrade(engine, order, bestSellOrder, bestSellOrder.RemainingQuantity)
			trades = append(trades, trade)

			order.RemainingQuantity -= bestSellOrder.RemainingQuantity
			order.Status = domain.StatusPartiallyFilled

			engine.book.ReduceBestSell(bestSellOrder.RemainingQuantity)

			nextBestSellOrder, nextBestSellOrderExists := engine.book.BestSell()
			if nextBestSellOrderExists && nextBestSellOrder.Price == order.Price{
				returnedOrder, nextTrades, err := submitBid(engine, order)
				return returnedOrder, append(trades, nextTrades...), err
			} else{
				engine.book.Add(*order)
				return order, trades, nil
			}
		}
	} else if order.Price > bestSellOrder.Price {
		for {
			if order.RemainingQuantity > bestSellOrder.RemainingQuantity {
				trade := newTrade(engine, order, bestSellOrder, bestSellOrder.RemainingQuantity)
				trades = append(trades, trade)

				order.RemainingQuantity -= bestSellOrder.RemainingQuantity
				order.Status = domain.StatusPartiallyFilled
				engine.book.RemoveBestSell()
			} else if order.RemainingQuantity <= bestSellOrder.RemainingQuantity {
				trade := newTrade(engine, order, bestSellOrder, order.RemainingQuantity)
				trades = append(trades, trade)

				engine.book.ReduceBestSell(order.RemainingQuantity)
				order.RemainingQuantity = 0
				order.Status = domain.StatusFilled
			}

			//next best sell order
			bestSellOrder, bestSellOrderExists = engine.book.BestSell()
			if !bestSellOrderExists ||
				order.Price < bestSellOrder.Price ||
				order.RemainingQuantity == 0 {
				break
			}
		}

		//if all sell orders are traded and buy order has remaining qty
		if bestSellOrder.RemainingQuantity == 0 && order.RemainingQuantity > 0 {
			engine.book.Add(*order)
		}

		return order, trades, nil
	}

	return order, trades, ErrUnhandledCase 
}

func SubmitAsk(engine *Engine, order *domain.Order) (*domain.Order, []domain.Trade, error) {
	trades := make([]domain.Trade, 0)

	return order, trades, ErrUnhandledCase 
}

func (engine *Engine) Submit(order *domain.Order) (*domain.Order, []domain.Trade, error) {
	if order.Side == domain.SideBuy {
		return submitBid(engine, order)
	} else if order.Side == domain.SideSell {
		return SubmitAsk(engine, order)
	}

	return order, make([]domain.Trade, 0), ErrUnhandledCase
}
