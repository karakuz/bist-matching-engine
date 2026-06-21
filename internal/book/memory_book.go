package book

import "bist-matching-engine/internal/domain"

type Order = domain.Order

// TODO: update bestBid and bestAsk on update/cancel order operations

type Book struct {
	// maps: lookup by price is fast but but best bid/ask discovery is slow (O(n) scan)
	buys  map[int64][]Order
	sells map[int64][]Order

	// hence keeping bestBid and bestAsk prices as attributes
	bestBid int64
	bestAsk int64
}

func (book *Book) Add(order Order) error {
	var sideLevels map[int64][]Order

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

func NewBook() *Book {
	return &Book{
		buys:  make(map[int64][]Order),
		sells: make(map[int64][]Order),
	}
}
