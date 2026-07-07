package domain

import (
	"encoding/json"
	"errors"
	"time"
)

type OrderEventType string

const (
	EventOrderAccepted        OrderEventType = "ORDER_ACCEPTED"
	EventOrderRejected        OrderEventType = "ORDER_REJECTED"
	EventOrderAddedToBook     OrderEventType = "ORDER_ADDED_TO_BOOK"
	EventOrderPartiallyFilled OrderEventType = "ORDER_PARTIALLY_FILLED"
	EventOrderFilled          OrderEventType = "ORDER_FILLED"
	EventTradeCreated         OrderEventType = "TRADE_CREATED"
)

var(
	ErrInvalidOrderEventType 	= errors.New("Invalid Order Event Type")
	ErrEmptyPayload 		 	= errors.New("Payload can not be empty")
	ErrInvalidPayload			= errors.New("Invalid payload")
)

type OrderEvent struct {
	ID        int64
	OrderID   string
	EventType OrderEventType
	Payload   []byte
	CreatedAt time.Time
}

func IsValidOrderEventType(eventType OrderEventType) bool {
	switch eventType {
		case EventOrderAccepted,
			EventOrderRejected,
			EventOrderAddedToBook,
			EventOrderPartiallyFilled,
			EventOrderFilled,
			EventTradeCreated:
			return true
		default:
			return false
	}
}

func NewOrderEvent(orderID string, eventType OrderEventType, payload []byte, createdAt time.Time) (*OrderEvent, error){
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}


	if !IsValidOrderEventType(eventType) {
		return nil, ErrInvalidOrderEventType
	}

	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}

	if !json.Valid(payload) {
		return nil, ErrInvalidPayload
	}

	return &OrderEvent{
		OrderID:   orderID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: createdAt,
	}, nil
}