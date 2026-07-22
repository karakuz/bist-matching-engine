package matching

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
)

var (
	ErrUnhandledCase                           = errors.New("Unhandled case error")
	ErrOrderSymbolDoesNotMatchWithEngineSymbol = errors.New("Order symbol does not match with engine symbol")
	ErrPriceOutsideAllowedRange                = errors.New("order price must be within the lower and upper price limits")
)

type Engine struct {
	mutex sync.RWMutex
	book *book.Book
}

func NewEngine(orderBook *book.Book) *Engine {
	return &Engine{book: orderBook}
}

func newTrade(engine *Engine, order *domain.Order, restingOrder domain.Order, quantity int64) domain.Trade {
	engine.book.UpdateLastTradePrice(restingOrder.Price)

	var BuyOrderID string
	var SellOrderID string
	if order.Side == domain.SideBuy {
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

func submitBid(engine *Engine, order *domain.Order) (MatchResult, error) {
	var err error = nil
	matchResult := newMatchResult(order)
	bestSellOrder, bestSellOrderExists := engine.book.BestSell()

	if !bestSellOrderExists {
		err = engine.book.Add(*order)
		return matchResult, err
	}

	if order.Price < bestSellOrder.Price {
		err = engine.book.Add(*order)
		return matchResult, err
	} else if order.Price == bestSellOrder.Price {
		for bestSellOrderExists && order.Price == bestSellOrder.Price && order.RemainingQuantity > 0 {
			tradeQty := min(order.RemainingQuantity, bestSellOrder.RemainingQuantity)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}

			updatedRestingOrder, exists :=engine.book.ReduceBestSell(tradeQty)
			if !exists {
				return matchResult, ErrUnhandledCase
			}
			matchResult.RestingOrderUpdates = append(matchResult.RestingOrderUpdates, updatedRestingOrder)

			trade := newTrade(engine, order, bestSellOrder, tradeQty)
			matchResult.Trades = append(matchResult.Trades, trade)

			bestSellOrder, bestSellOrderExists = engine.book.BestSell()
		}

		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}

		return matchResult, err
	} else if order.Price > bestSellOrder.Price {
		for bestSellOrderExists &&
			order.Price >= bestSellOrder.Price &&
			order.RemainingQuantity != 0 {

			tradeQty := min(bestSellOrder.RemainingQuantity, order.RemainingQuantity)

			trade := newTrade(engine, order, bestSellOrder, tradeQty)
			matchResult.Trades = append(matchResult.Trades, trade)
			
			updatedRestingOrder, exists := engine.book.ReduceBestSell(tradeQty)
			if !exists {
				return matchResult, ErrUnhandledCase
			}
			matchResult.RestingOrderUpdates = append(matchResult.RestingOrderUpdates, updatedRestingOrder)

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

		return matchResult, err
	}

	return matchResult, ErrUnhandledCase
}

func submitAsk(engine *Engine, order *domain.Order) (MatchResult, error) {
	var err error = nil
	matchResult := newMatchResult(order)
	bestBuyOrder, bestBuyOrderExists := engine.book.BestBuy()

	if !bestBuyOrderExists {
		err = engine.book.Add(*order)
		return matchResult, err
	}

	if order.Price > bestBuyOrder.Price {
		err = engine.book.Add(*order)
		return matchResult, err
	} else if order.Price == bestBuyOrder.Price {
		for bestBuyOrderExists && order.Price <= bestBuyOrder.Price && order.RemainingQuantity > 0 {
			tradeQty := min(order.RemainingQuantity, bestBuyOrder.RemainingQuantity)

			order.RemainingQuantity -= tradeQty
			if order.RemainingQuantity == 0 {
				order.Status = domain.StatusFilled
			} else {
				order.Status = domain.StatusPartiallyFilled
			}
			updatedRestingOrder, exists := engine.book.ReduceBestBuy(tradeQty)
			if !exists {
				return matchResult, ErrUnhandledCase
			}
			matchResult.RestingOrderUpdates = append(matchResult.RestingOrderUpdates, updatedRestingOrder)

			trade := newTrade(engine, order, bestBuyOrder, tradeQty)
			matchResult.Trades = append(matchResult.Trades, trade)

			bestBuyOrder, bestBuyOrderExists = engine.book.BestBuy()
		}

		if order.RemainingQuantity > 0 {
			err = engine.book.Add(*order)
		}

		return matchResult, err
	} else if order.Price < bestBuyOrder.Price {
		for bestBuyOrderExists &&
			order.Price <= bestBuyOrder.Price &&
			order.RemainingQuantity != 0 {

			tradeQty := min(bestBuyOrder.RemainingQuantity, order.RemainingQuantity)

			trade := newTrade(engine, order, bestBuyOrder, tradeQty)
			matchResult.Trades = append(matchResult.Trades, trade)

			updatedRestingOrder, exists := engine.book.ReduceBestBuy(tradeQty)
			if !exists {
				return matchResult, ErrUnhandledCase
			}
			matchResult.RestingOrderUpdates = append(matchResult.RestingOrderUpdates, updatedRestingOrder)

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
		return matchResult, err
	}

	return matchResult, ErrUnhandledCase
}

type MatchResult struct {
	IncomingOrder       *domain.Order
	RestingOrderUpdates []domain.Order
	Trades              []domain.Trade
}

func newMatchResult(order *domain.Order) MatchResult{
	return MatchResult{
		IncomingOrder: order,
		RestingOrderUpdates: make([]domain.Order, 0),
		Trades: make([]domain.Trade, 0),
	}
}

func (engine *Engine) Submit(order *domain.Order) (MatchResult, error) { //(*domain.Order, []domain.Trade, error)
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	matchResult := newMatchResult(order)

	if order.Symbol.Code != engine.book.Symbol.Code {
		return matchResult, ErrOrderSymbolDoesNotMatchWithEngineSymbol
	}
	upperPriceLimit := engine.book.GetUpperPriceLimit()
	lowerPriceLimit := engine.book.GetLowerPriceLimit()

	isValidPrice := upperPriceLimit >= order.Price && lowerPriceLimit <= order.Price
	if !isValidPrice {
		return matchResult, ErrPriceOutsideAllowedRange
	}

	if order.Side == domain.SideBuy {
		return submitBid(engine, order)
	} else if order.Side == domain.SideSell {
		return submitAsk(engine, order)
	}

	return matchResult, ErrUnhandledCase
}

func (engine *Engine) SessionDate() time.Time{
	return engine.book.SessionDate
}

func (engine *Engine) Snapshot(levels int64) (book.Snapshot, error){
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	return engine.book.Snapshot(levels)
}