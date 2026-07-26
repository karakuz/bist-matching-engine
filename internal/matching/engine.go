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
	book  *book.Book
}

func NewEngine(orderBook *book.Book) *Engine {
	return &Engine{book: orderBook}
}

func newTrade(order domain.Order, restingOrder domain.Order, quantity int64) domain.Trade {
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

			updatedRestingOrder, exists := engine.book.ReduceBestSell(tradeQty)
			if !exists {
				return matchResult, ErrUnhandledCase
			}
			matchResult.RestingOrderUpdates = append(matchResult.RestingOrderUpdates, updatedRestingOrder)

			trade := newTrade(*order, bestSellOrder, tradeQty)
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

			trade := newTrade(*order, bestSellOrder, tradeQty)
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

			trade := newTrade(*order, bestBuyOrder, tradeQty)
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

			trade := newTrade(*order, bestBuyOrder, tradeQty)
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

func newMatchResult(order *domain.Order) MatchResult {
	return MatchResult{
		IncomingOrder:       order,
		RestingOrderUpdates: make([]domain.Order, 0),
		Trades:              make([]domain.Trade, 0),
	}
}

func (engine *Engine) Submit(order *domain.Order) (MatchResult, error) {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	result, err := engine.submit(order)
	if err != nil {
		return result, err
	}

	if len(result.Trades) > 0 {
		lastTrade := result.Trades[len(result.Trades)-1]
		engine.book.UpdateLastTradePrice(lastTrade.Price)
	}

	return result, nil
}

func (engine *Engine) submit(order *domain.Order) (MatchResult, error) {
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

	switch order.Side {
	case domain.SideBuy:
		return submitBid(engine, order)
	case domain.SideSell:
		return submitAsk(engine, order)
	}

	return matchResult, ErrUnhandledCase
}

func (engine *Engine) SessionDate() time.Time {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	return engine.book.SessionDate
}

func (engine *Engine) Snapshot(levels int64) (book.Snapshot, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	return engine.book.Snapshot(levels)
}

type MatchPlan struct {
	IncomingOrder       domain.Order
	RestingOrderUpdates []domain.Order
	Trades              []domain.Trade
}

func (engine *Engine) Prepare(order domain.Order) (MatchPlan, error) {
	engine.mutex.RLock()
	defer engine.mutex.RUnlock()

	if order.Symbol.Code != engine.book.Symbol.Code {
		return MatchPlan{},	ErrOrderSymbolDoesNotMatchWithEngineSymbol
	}

	if order.Price < engine.book.GetLowerPriceLimit() ||
		order.Price > engine.book.GetUpperPriceLimit() {
		return MatchPlan{}, ErrPriceOutsideAllowedRange
	}

	candidates, err := engine.book.MatchCandidates(order)
	if err != nil {
		return MatchPlan{}, err
	}

	incomingOrder := order
	restingOrderUpdates := make([]domain.Order, 0, len(candidates))
	trades := make([]domain.Trade, 0, len(candidates))

	for _, candidate := range candidates {
		if incomingOrder.RemainingQuantity == 0 {
			break
		}

		tradeQuantity := min(incomingOrder.RemainingQuantity,candidate.RemainingQuantity)

		incomingOrder.RemainingQuantity -= tradeQuantity
		candidate.RemainingQuantity -= tradeQuantity

		if incomingOrder.RemainingQuantity == 0 {
			incomingOrder.Status = domain.StatusFilled
		} else {
			incomingOrder.Status = domain.StatusPartiallyFilled
		}

		if candidate.RemainingQuantity == 0 {
			candidate.Status = domain.StatusFilled
		} else {
			candidate.Status = domain.StatusPartiallyFilled
		}

		restingOrderUpdates = append(restingOrderUpdates,candidate)

		trade := newTrade(incomingOrder,candidate,tradeQuantity)
		trades = append(trades,trade)
	}

	return MatchPlan{
		IncomingOrder:       incomingOrder,
		RestingOrderUpdates: restingOrderUpdates,
		Trades:              trades,
	}, nil
}

func (engine *Engine) Apply(plan MatchPlan) error {
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	for index, trade := range plan.Trades {
		var (
			updatedOrder domain.Order
			exists       bool
		)

		switch plan.IncomingOrder.Side {
		case domain.SideBuy:
			updatedOrder, exists =
				engine.book.ReduceBestSell(trade.Quantity)

		case domain.SideSell:
			updatedOrder, exists =
				engine.book.ReduceBestBuy(trade.Quantity)

		default:
			return domain.ErrInvalidSide
		}

		if !exists {
			return ErrUnhandledCase
		}

		expectedUpdate :=
			plan.RestingOrderUpdates[index]

		if updatedOrder.ID != expectedUpdate.ID ||
			updatedOrder.RemainingQuantity !=
				expectedUpdate.RemainingQuantity ||
			updatedOrder.Status != expectedUpdate.Status {
			return ErrUnhandledCase
		}
	}

	if plan.IncomingOrder.RemainingQuantity > 0 {
		if err := engine.book.Add(
			plan.IncomingOrder,
		); err != nil {
			return err
		}
	}

	if len(plan.Trades) > 0 {
		lastTrade := plan.Trades[len(plan.Trades)-1]

		engine.book.UpdateLastTradePrice(
			lastTrade.Price,
		)
	}

	return nil
}