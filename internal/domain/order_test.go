package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var testParticipantId int64 = 1

func TestNewOrder_RejectsInvalidInput(t *testing.T) {
	symbol, err := NewSymbol("ASELS", 10)
	if err != nil {
		t.Fatalf("NewSymbol failed: %v", err)
	}

	tests := []struct {
		name     string
		id       string
		symbol   Symbol
		side     Side
		price    int64
		quantity int64
		wantErr  error
	}{
		{
			name:     "empty id",
			id:       "",
			symbol:   symbol,
			side:     SideBuy,
			price:    1050,
			quantity: 100,
			wantErr:  ErrEmptyOrderID,
		},
		{
			name:     "invalid side",
			id:       uuid.NewString(),
			symbol:   symbol,
			side:     "HOLD",
			price:    1050,
			quantity: 100,
			wantErr:  ErrInvalidSide,
		},
		{
			name:     "price <= 0",
			id:       uuid.NewString(),
			symbol:   symbol,
			side:     SideBuy,
			price:    0,
			quantity: 100,
			wantErr:  ErrInvalidPrice,
		},
		{
			name:     "quantity <= 0",
			id:       uuid.NewString(),
			symbol:   symbol,
			side:     SideBuy,
			price:    1050,
			quantity: 0,
			wantErr:  ErrInvalidQty,
		},
		{
			name:     "invalid tick size",
			id:       uuid.NewString(),
			symbol:   symbol,
			side:     SideBuy,
			price:    1051,
			quantity: 1,
			wantErr:  ErrInvalidTickSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOrder(tt.id, testParticipantId, tt.symbol, tt.side, tt.price, tt.quantity, time.Now().UTC())
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewOrder_AcceptsValidOrder(t *testing.T) {
	var testParticipantId int64 = 1

	symbol, err := NewSymbol("ASELS", 10)
	now := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

	id := uuid.NewString()
	order, err := NewOrder(id, testParticipantId, symbol, SideBuy, 1050, 100, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if order.ID != id {
		t.Fatalf("unexpected ID: %s", order.ID)
	}
	if order.Symbol.Code != "ASELS" {
		t.Fatalf("unexpected Symbol: %s", order.Symbol.Code)
	}
	if order.Side != SideBuy {
		t.Fatalf("unexpected Side: %s", order.Side)
	}
	if order.Price != 1050 {
		t.Fatalf("unexpected Price: %d", order.Price)
	}
	if order.Quantity != 100 {
		t.Fatalf("unexpected Quantity: %d", order.Quantity)
	}
	if order.RemainingQuantity != 100 {
		t.Fatalf("unexpected RemainingQuantity: %d", order.RemainingQuantity)
	}
	if order.Status != StatusOpen {
		t.Fatalf("unexpected Status: %s", order.Status)
	}
	if !order.CreatedAt.Equal(now) {
		t.Fatalf("unexpected CreatedAt: %v", order.CreatedAt)
	}
}
