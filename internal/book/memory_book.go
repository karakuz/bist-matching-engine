package book

import (
	"errors"
	"bist-matching-engine/internal/domain"
)

type Order = domain.Order
type Symbol = domain.Symbol

var (
	ErrInvalidOrderSymbol    = errors.New("Order symbol does not match with Book's symbol")
)

// TODO: update bestBid and bestAsk on update/cancel order operations

type Book struct {
	Symbol Symbol

	lastTradePrice int64
	
	// maps: lookup by price is fast but but best bid/ask discovery is slow (O(n) scan)
	buys  map[int64][]Order
	sells map[int64][]Order

	// hence keeping bestBid and bestAsk prices as attributes
	bestBid int64//buy
	bestAsk int64//sell
}

func (book *Book) Add(orders ...Order) error {
	for _, order := range orders{
		var sideLevels map[int64][]Order

		if order.Symbol.Code != book.Symbol.Code{
			return ErrInvalidOrderSymbol
		}

		if order.Side == domain.SideBuy {
			sideLevels = book.buys
			if book.bestBid == 0 || order.Price > book.bestBid {
				book.bestBid = order.Price
			}
		} else if order.Side == domain.SideSell {
			sideLevels = book.sells
			if book.bestAsk == 0 || order.Price < book.bestAsk {
				book.bestAsk = order.Price
			}
		} else {
			return domain.ErrInvalidSide
		}

		sideLevels[order.Price] = append(sideLevels[order.Price], order)
	}
	

	return nil
}

func (book *Book) BestBuy() (domain.Order, bool) {
	bestBid := book.bestBid
	if bestBid == 0 {
		return Order{}, false
	}

	level, exists := book.buys[bestBid]

	if !exists || len(level) == 0 {
		return Order{}, false
	}

	//return the oldest Order
	return level[0], true
}

func (book *Book) recalculateBestBuy() {
	var bestBuy int64 = 0
	for price := range book.buys {
		if price > bestBuy {
			bestBuy = price
		}
	}
	book.bestBid = bestBuy
}

func (book *Book) recalculateBestSell() {
	var bestSell int64
	firstIter := true
	for price := range book.sells {
		if firstIter || price < bestSell {
			bestSell = price
			firstIter = false
		}
	}
	book.bestAsk = bestSell
}

func (book *Book) RemoveBestBuy() {
	bestBuyOrder, bestBuyExists := book.BestBuy()

	if !bestBuyExists {
		return
	}

	level := book.buys[bestBuyOrder.Price]
	newLevel := level[1:]
	if len(newLevel) == 0 {
		delete(book.buys, bestBuyOrder.Price)
		book.recalculateBestBuy()
	} else {
		book.buys[bestBuyOrder.Price] = newLevel
	}
}

func (book *Book) ReduceBestBuy(quantity int64) {
	bestBuyOrder, exists := book.BestBuy()
	if !exists {
		return
	}

	level := book.buys[bestBuyOrder.Price]
	if len(level) == 0 {
		return
	}
	firstBid := &level[0]
	quantityCanBeReduced := min(firstBid.RemainingQuantity, quantity)
	firstBid.RemainingQuantity -= quantityCanBeReduced

	if firstBid.RemainingQuantity == 0 {
		firstBid.Status = domain.StatusFilled
		book.RemoveBestBuy()
		return
	}

	level[0].Status = domain.StatusPartiallyFilled
	book.buys[bestBuyOrder.Price] = level
}

func (book *Book) BestSell() (domain.Order, bool) {
	bestAsk := book.bestAsk
	if bestAsk == 0 {
		return Order{}, false
	}

	level, exists := book.sells[bestAsk]

	if !exists || len(level) == 0 {
		return Order{}, false
	}

	//return the oldest Order
	return level[0], true
}

func (book *Book) RemoveBestSell() {
	bestSellOrder, bestSellExists := book.BestSell()

	if !bestSellExists {
		return
	}

	level := book.sells[bestSellOrder.Price]
	newLevel := level[1:]
	if len(newLevel) == 0 {
		delete(book.sells, bestSellOrder.Price)
		book.recalculateBestSell()
	} else {
		book.sells[bestSellOrder.Price] = newLevel
	}
}

func (book *Book) ReduceBestSell(quantity int64) {
	bestSellOrder, exists := book.BestSell()
	if !exists {
		return
	}

	level := book.sells[bestSellOrder.Price]
	if len(level) == 0 {
		return
	}
	firstAsk := &level[0]
	quantityCanBeReduced := min(firstAsk.RemainingQuantity, quantity)
	firstAsk.RemainingQuantity -= quantityCanBeReduced

	if firstAsk.RemainingQuantity == 0 {
		book.RemoveBestSell()
		return
	}

	firstAsk.Status = domain.StatusPartiallyFilled
	book.sells[bestSellOrder.Price] = level
}

func NewBook(symbol Symbol) *Book {
	return &Book{
		Symbol: symbol,
		lastTradePrice: 0,
		buys:  make(map[int64][]Order),
		sells: make(map[int64][]Order),
	}
}

func (book *Book) GetLevel(side domain.Side, price int64) []Order{
	if side == domain.SideBuy {
		return book.buys[price]
	}

	return book.sells[price]
}

func (book *Book) GetLastTradePrice() int64{
	return book.lastTradePrice
}

func (book *Book) UpdateLastTradePrice(price int64){
	book.lastTradePrice = price
}