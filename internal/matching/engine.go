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
			// todo
			return order, []domain.Trade{}, nil
		}

		if order.Price < bestSellOrder.Price {
			// todo
			return order, []domain.Trade{}, nil
		} else if order.Price == bestSellOrder.Price {
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

				engine.book.ReduceBestSell(order.RemainingQuantity)

				order.RemainingQuantity = 0
				order.Status = domain.StatusFilled

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
				engine.book.Add(*order)

				return order, trades, nil
			}
		} else if order.Price > bestSellOrder.Price {
			for {
				if order.RemainingQuantity > bestSellOrder.RemainingQuantity {
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

					order.RemainingQuantity -= bestSellOrder.RemainingQuantity
					engine.book.RemoveBestSell()
				} else if order.RemainingQuantity <= bestSellOrder.RemainingQuantity {
					trade := domain.Trade{
						ID:          uuid.NewString(),
						Symbol:      order.Symbol,
						BuyOrderID:  order.ID,
						SellOrderID: bestSellOrder.ID,
						Price:       bestSellOrder.Price,
						Quantity:    order.RemainingQuantity,
						CreatedAt:   time.Now().UTC(),
					}
					trades = append(trades, trade)

					engine.book.ReduceBestSell(order.RemainingQuantity)
					order.RemainingQuantity = 0
			 	}

				bestSellOrder, bestSellOrderExists = engine.book.BestSell()
				if !bestSellOrderExists ||
					order.Price < bestSellOrder.Price ||
					order.RemainingQuantity == 0 {
					if order.RemainingQuantity == 0 {
						order.Status = domain.StatusFilled
					} else {
						order.Status = domain.StatusPartiallyFilled
					}
					break
				}
			}

			return order, trades, nil
		}
	} else if order.Side == domain.SideSell {

	}

	return &domain.Order{}, trades, nil
}
