package book

import (
	"bist-matching-engine/internal/domain"
	"errors"
	"fmt"
	"slices"
	"time"
)

type Order = domain.Order
type Symbol = domain.Symbol

const MAX_SNAPSHOT_LEVELS = 100

var (
	ErrInvalidAddSideConflict   = errors.New("Book has different side than current one on this level")
	ErrInvalidOrderSymbol       = errors.New("Order symbol does not match with Book's symbol")
	ErrPriceOutsideAllowedRange = errors.New("order price must be within the lower and upper price limits")

	ErrSnapshotSizeNonPositive                = errors.New("Snapshot size must be > 0")
	ErrRequestedMoreLevelsThanMaxSnapshotSize = fmt.Errorf("Snapshot size can not exceed %v", MAX_SNAPSHOT_LEVELS)
)

type Book struct {
	Symbol      Symbol
	SessionDate time.Time

	lastTradePrice int64
	openingPrice   int64

	upperPriceLimit int64
	lowerPriceLimit int64

	// maps: lookup by price is fast but but best bid/ask discovery is slow (O(n) scan)
	buys  map[int64][]Order
	sells map[int64][]Order

	// hence keeping bestBid and bestAsk prices as attributes
	bestBid int64 //buy
	bestAsk int64 //sell
}

func (book *Book) Add(orders ...Order) error {
	//check all order prices - whether they fit within limits
	for _, order := range orders {
		isWithinLimit := book.lowerPriceLimit <= order.Price && book.upperPriceLimit >= order.Price
		if !isWithinLimit {
			return ErrPriceOutsideAllowedRange
		}
	}

	for _, order := range orders {
		var sideLevels map[int64][]Order

		if order.Symbol.Code != book.Symbol.Code {
			return ErrInvalidOrderSymbol
		}

		switch order.Side {
		case domain.SideBuy:
			//check if other side has same level
			if len(book.sells[order.Price]) > 0 {
				return ErrInvalidAddSideConflict
			}

			sideLevels = book.buys
			if book.bestBid == 0 || order.Price > book.bestBid {
				book.bestBid = order.Price
			}
		case domain.SideSell:
			//check if other side has same level
			if len(book.buys[order.Price]) > 0 {
				return ErrInvalidAddSideConflict
			}

			sideLevels = book.sells
			if book.bestAsk == 0 || order.Price < book.bestAsk {
				book.bestAsk = order.Price
			}
		default:
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

func (book *Book) removeBestBuy() {
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

func (book *Book) ReduceBestBuy(quantity int64) (domain.Order, bool) {
	bestBuyOrder, exists := book.BestBuy()
	if !exists {
		return Order{}, false
	}

	level := book.buys[bestBuyOrder.Price]
	if len(level) == 0 {
		return Order{}, false
	}
	firstBid := &level[0]
	quantityToReduce := min(firstBid.RemainingQuantity, quantity)
	firstBid.RemainingQuantity -= quantityToReduce

	if firstBid.RemainingQuantity == 0 {
		firstBid.Status = domain.StatusFilled
		updatedOrder := *firstBid

		book.removeBestBuy()

		return updatedOrder, true
	}

	level[0].Status = domain.StatusPartiallyFilled
	book.buys[bestBuyOrder.Price] = level

	return *firstBid, true
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

func (book *Book) removeBestSell() {
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

func (book *Book) ReduceBestSell(quantity int64) (domain.Order, bool) {
	bestSellOrder, exists := book.BestSell()
	if !exists {
		return domain.Order{}, false
	}

	level := book.sells[bestSellOrder.Price]
	if len(level) == 0 {
		return domain.Order{}, false
	}
	firstAsk := &level[0]
	quantityToReduce := min(firstAsk.RemainingQuantity, quantity)
	firstAsk.RemainingQuantity -= quantityToReduce

	if firstAsk.RemainingQuantity == 0 {
		firstAsk.Status = domain.StatusFilled
		updatedOrder := *firstAsk

		book.removeBestSell()
		return updatedOrder, true
	}

	firstAsk.Status = domain.StatusPartiallyFilled
	book.sells[bestSellOrder.Price] = level

	return *firstAsk, true
}

func calculatePriceLimits(openingPrice, tickSize int64) (upper, lower int64) {
	const percentageBase int64 = 100
	const priceLimitPercentage int64 = 10

	tickDenominator := percentageBase * tickSize

	upperNumerator := openingPrice * (percentageBase + priceLimitPercentage)
	lowerNumerator := openingPrice * (percentageBase - priceLimitPercentage)

	upper = (upperNumerator / tickDenominator) * tickSize

	lower = ((lowerNumerator + tickDenominator - 1) / tickDenominator) * tickSize

	return upper, lower
}

func NewBook(symbol Symbol, sessionDate time.Time, openingPrice int64) *Book {
	upperPriceLimit, lowerPriceLimit := calculatePriceLimits(openingPrice, symbol.TickSize)

	return &Book{
		Symbol:          symbol,
		SessionDate:     sessionDate,
		lastTradePrice:  openingPrice,
		openingPrice:    openingPrice,
		upperPriceLimit: upperPriceLimit,
		lowerPriceLimit: lowerPriceLimit,
		buys:            make(map[int64][]Order),
		sells:           make(map[int64][]Order),
	}
}

type PriceLevel struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

type Snapshot struct {
	Symbol string       `json:"symbol"`
	Buy    []PriceLevel `json:"buy"`
	Sell   []PriceLevel `json:"sell"`
}

func (book *Book) Snapshot(levels int64) (Snapshot, error) {
	if levels < 1 {
		return Snapshot{}, ErrSnapshotSizeNonPositive
	}
	if levels > MAX_SNAPSHOT_LEVELS {
		return Snapshot{}, ErrRequestedMoreLevelsThanMaxSnapshotSize
	}
	snapshot := Snapshot{
		Symbol: book.Symbol.Code,
		Buy:    make([]PriceLevel, 0, levels),
		Sell:   make([]PriceLevel, 0, levels),
	}

	buyPrices := make([]int64, 0, levels)
	for buyPrice := range book.buys {
		buyPrices = append(buyPrices, buyPrice)
	}

	//sort descending
	slices.Sort(buyPrices)
	slices.Reverse(buyPrices)

	for _, buyPrice := range buyPrices {
		levelOrders := book.buys[buyPrice]

		priceLevel := PriceLevel{
			Price:    buyPrice,
			Quantity: 0,
		}
		for _, levelOrder := range levelOrders {
			priceLevel.Quantity += levelOrder.RemainingQuantity
		}
		snapshot.Buy = append(snapshot.Buy, priceLevel)

		if int64(len(snapshot.Buy)) == levels {
			break
		}
	}

	sellPrices := make([]int64, 0, levels)
	for sellPrice := range book.sells {
		sellPrices = append(sellPrices, sellPrice)
	}

	//sort ascending
	slices.Sort(sellPrices)

	for _, sellPrice := range sellPrices {
		levelOrders := book.sells[sellPrice]

		priceLevel := PriceLevel{
			Price:    sellPrice,
			Quantity: 0,
		}
		for _, levelOrder := range levelOrders {
			priceLevel.Quantity += levelOrder.RemainingQuantity
		}
		snapshot.Sell = append(snapshot.Sell, priceLevel)

		if int64(len(snapshot.Sell)) == levels {
			break
		}
	}

	return snapshot, nil
}

func (book *Book) GetLevel(side domain.Side, price int64) []Order {
	if side == domain.SideBuy {
		return book.buys[price]
	}

	return book.sells[price]
}

func (book *Book) GetLastTradePrice() int64 {
	return book.lastTradePrice
}

func (book *Book) UpdateLastTradePrice(price int64) {
	book.lastTradePrice = price
}

func (book *Book) GetUpperPriceLimit() int64 {
	return book.upperPriceLimit
}

func (book *Book) GetLowerPriceLimit() int64 {
	return book.lowerPriceLimit
}

func (book *Book) MatchCandidates(order Order) ([]Order, error) {
	incomingSide := order.Side
	incomingPrice := order.Price
	
	var prices []int64
	var levels map[int64][]Order

	switch incomingSide {
	case domain.SideBuy:
		levels = book.sells

		for price := range levels {
			if price <= incomingPrice {
				prices = append(prices, price)
			}
		}

		slices.Sort(prices)

	case domain.SideSell:
		levels = book.buys

		for price := range levels {
			if price >= incomingPrice {
				prices = append(prices, price)
			}
		}

		slices.Sort(prices)
		slices.Reverse(prices)

	default:
		return nil, domain.ErrInvalidSide
	}

	candidates := make([]Order, 0)
	remainingMatches  := order.RemainingQuantity

	if remainingMatches <= 0 {
		return candidates, nil
	}

	for _, candidatePrice := range prices {
		for _, order := range levels[candidatePrice] {
			if order.RemainingQuantity <= 0 {
				continue
			}

			candidates = append(candidates, order)

			matchedQuantity := min(
				remainingMatches,
				order.RemainingQuantity,
			)

			remainingMatches -= matchedQuantity

			if remainingMatches == 0 {
				return candidates, nil
			}
		}
	}

	return candidates, nil
}