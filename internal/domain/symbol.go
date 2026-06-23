package domain

import (
	"errors"
	"strings"
)

var (
	ErrEmptySymbol    = errors.New("symbol cannot be empty")
	ErrInvalidSymbol  = errors.New("symbol code must be 5 characters")
	ErrInvalidTickSize    = errors.New("symbol cannot be empty")
)

type Symbol struct{
	ID string
	Code string
	TickSize int64
}

func (symbol *Symbol) ValidateTickOfPrice(price int64) error {
	if price % symbol.TickSize != 0 {
		return ErrInvalidTickSize
	}
	return nil
}

func NewSymbol(code string, tickSize int64) (Symbol, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	
	if code == "" {
		return Symbol{}, ErrEmptySymbol
	}

	if len(code) != 5 {
		return Symbol{}, ErrInvalidSymbol
	}

	if tickSize < 1 {
		return Symbol{}, ErrInvalidTickSize
	}
	
	return Symbol{
		Code: strings.ToUpper(code),
		TickSize: tickSize,
	}, nil
}