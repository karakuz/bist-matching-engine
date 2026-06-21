package matching

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/domain"
)

// TODO: implement submit + matching logic (price-time priority)
// TODO: create trades using resting order price

var (
	ErrTest = errors.New("test")
)

type Engine struct {
	book *book.Book
}

func NewEngine(orderBook *book.Book) *Engine {
	return &Engine{book: orderBook}
}

/*

 */

func (engine *Engine) Submit(order *domain.Order) (*domain.Order, []domain.Trade, error) {
	trades := make([]domain.Trade, 0)

	if order.Side == domain.SideBuy {
		bestSellOrder, bestSellOrderExists := engine.book.BestSell()

		if !bestSellOrderExists {
			return order, []domain.Trade{}, nil
		}
		if order.Price < bestSellOrder.Price {
			return order, []domain.Trade{}, nil
		}

		if order.Price == bestSellOrder.Price {
			if bestSellOrder.RemainingQuantity >= order.Quantity {
				trade := domain.Trade{
					ID:          uuid.NewString(),
					Symbol:      order.Symbol,
					BuyOrderID:  order.ID,
					SellOrderID: bestSellOrder.ID,
					Price:       bestSellOrder.Price,
					Quantity:    order.Quantity,
					CreatedAt:   time.Now().UTC(),
				}
				trades = append(trades, trade)
				
				order.RemainingQuantity = 0
				order.Status = domain.StatusFilled

				//why ???
				engine.book.RemoveBestSell()

				return order, trades, nil
			} else if bestSellOrder.RemainingQuantity < order.Quantity {
				trade := domain.Trade{
					ID:          uuid.NewString(),
					Symbol:      order.Symbol,
					BuyOrderID:  order.ID,
					SellOrderID: bestSellOrder.ID,
					Price:       bestSellOrder.Price,
					Quantity:    bestSellOrder.RemainingQuantity,
					CreatedAt:   time.Now().UTC(),
				}
				trades = append(trades, trade)

				order.RemainingQuantity = order.Quantity - bestSellOrder.RemainingQuantity
				order.Status = domain.StatusPartiallyFilled

				engine.book.RemoveBestSell()
			}
		} else if order.Price > bestSellOrder.Price {
			for{
				var tradeQuantity int64
				var orderRemainingQuantity int64
				var newOrderStatus domain.OrderStatus

				if order.RemainingQuantity > bestSellOrder.RemainingQuantity {
					tradeQuantity = bestSellOrder.RemainingQuantity
					orderRemainingQuantity = order.RemainingQuantity - bestSellOrder.RemainingQuantity
					newOrderStatus = domain.StatusFilled	
				} else if order.RemainingQuantity <= bestSellOrder.RemainingQuantity {
					tradeQuantity = order.Quantity
					orderRemainingQuantity = order.Quantity
					newOrderStatus = domain.StatusFilled	
				}

				trade := domain.Trade{
					ID:          uuid.NewString(),
					Symbol:      order.Symbol,
					BuyOrderID:  order.ID,
					SellOrderID: bestSellOrder.ID,
					Price:       bestSellOrder.Price,
					Quantity:    tradeQuantity,
					CreatedAt:   time.Now().UTC(),
				}
				trades = append(trades, trade)
				
				order.RemainingQuantity = orderRemainingQuantity
				order.Status = newOrderStatus
				engine.book.RemoveBestSell()

				bestSellOrder, bestSellOrderExists := engine.book.BestSell()
				if !bestSellOrderExists || order.Price > bestSellOrder.Price || order.RemainingQuantity <= bestSellOrder.RemainingQuantity{
					break
				}
			}
		}
	} else if order.Side == domain.SideSell {

	}

	return &domain.Order{}, trades, nil
}
