package domain

import (
	"errors"
	"strings"
	"time"
)

// "BUY" || "SELL"
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type OrderStatus string

const (
	StatusOpen            OrderStatus = "OPEN"
	StatusPartiallyFilled OrderStatus = "PARTIALLY_FILLED"
	StatusFilled          OrderStatus = "FILLED"
	StatusRejected        OrderStatus = "REJECTED"
)

type Order struct {
	ID                string
	ParticipantID     int64
	Symbol            Symbol
	Side              Side
	Price             int64
	Quantity          int64
	RemainingQuantity int64
	Status            OrderStatus
	CreatedAt         time.Time
}

type Trade struct {
	ID          string
	Symbol      Symbol
	BuyOrderID  string
	SellOrderID string
	Price       int64
	Quantity    int64
	CreatedAt   time.Time
}

var (
	ErrInvalidSide  = errors.New("side must be BUY or SELL")
	ErrInvalidPrice = errors.New("price must be > 0")
	ErrInvalidQty   = errors.New("quantity must be > 0")
	ErrEmptyOrderID = errors.New("order id cannot be empty")
)

func NewOrder(
	id string, 
	participantID int64,
	symbol Symbol, 
	side Side, 
	price, 
	quantity int64, 
	createdAt time.Time,
	) (Order, error) {
	if strings.TrimSpace(id) == "" {
		return Order{}, ErrEmptyOrderID
	}

	err := symbol.ValidateTickOfPrice(price)
	if err != nil {
		return Order{}, err
	}

	if side != SideBuy && side != SideSell {
		return Order{}, ErrInvalidSide
	}
	if price <= 0 {
		return Order{}, ErrInvalidPrice
	}
	if quantity <= 0 {
		return Order{}, ErrInvalidQty
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return Order{
		ID:                id,
		ParticipantID:     participantID,
		Symbol:            symbol,
		Side:              side,
		Price:             price,
		Quantity:          quantity,
		RemainingQuantity: quantity,
		Status:            StatusOpen,
		CreatedAt:         createdAt,
	}, nil
}
