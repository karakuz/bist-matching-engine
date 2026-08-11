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

	return engine.prepare(order)
}

func (engine *Engine) prepare(order domain.Order) (MatchPlan, error) {

	if order.Symbol.Code != engine.book.Symbol.Code {
		return MatchPlan{}, ErrOrderSymbolDoesNotMatchWithEngineSymbol
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

		tradeQuantity := min(incomingOrder.RemainingQuantity, candidate.RemainingQuantity)

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

		restingOrderUpdates = append(restingOrderUpdates, candidate)

		trade := newTrade(incomingOrder, candidate, tradeQuantity)
		trades = append(trades, trade)
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

	return engine.apply(plan)
}

func (engine *Engine) apply(plan MatchPlan) error {

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
