package domain

import "testing"

func TestNewSymbol_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		tickSize int64
		wantErr  error
	}{
		{
			name:     "empty code",
			code:     "",
			tickSize: 1,
			wantErr:  ErrEmptySymbol,
		},
		{
			name:     "blank code",
			code:     "   ",
			tickSize: 1,
			wantErr:  ErrEmptySymbol,
		},
		{
			name:     "code shorter than five characters",
			code:     "ASEL",
			tickSize: 1,
			wantErr:  ErrInvalidSymbol,
		},
		{
			name:     "code longer than five characters",
			code:     "ASELSS",
			tickSize: 1,
			wantErr:  ErrInvalidSymbol,
		},
		{
			name:     "zero tick size",
			code:     "ASELS",
			tickSize: 0,
			wantErr:  ErrInvalidTickSize,
		},
		{
			name:     "negative tick size",
			code:     "ASELS",
			tickSize: -1,
			wantErr:  ErrInvalidTickSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSymbol(tt.code, tt.tickSize)
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewSymbol_AcceptsValidSymbol(t *testing.T) {
	symbol, err := NewSymbol(" asels ", 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if symbol.Code != "ASELS" {
		t.Fatalf("expected code ASELS, got %s", symbol.Code)
	}
	if symbol.TickSize != 5 {
		t.Fatalf("expected tick size 5, got %d", symbol.TickSize)
	}
}

func TestSymbol_ValidateTickOfPrice(t *testing.T) {
	symbol, err := NewSymbol("ASELS", 5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	tests := []struct {
		name    string
		price   int64
		wantErr error
	}{
		{
			name:    "price aligned with tick size",
			price:   1000,
			wantErr: nil,
		},
		{
			name:    "price not aligned with tick size",
			price:   1001,
			wantErr: ErrInvalidTickSize,
		},
		{
			name:    "another aligned price",
			price:   1010,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := symbol.ValidateTickOfPrice(tt.price)
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
