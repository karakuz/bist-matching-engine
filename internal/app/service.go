package app

import (
	"errors"
	"strings"


	"bist-matching-engine/internal/domain"
)

var ErrInvalidOrder = errors.New("invalid order")


type SubmitOrderRequest struct {
	ParticipantId int64       `json:"participantId"`
	Symbol        string      `json:"symbol"`
	Side          domain.Side `json:"side"`
	Price         int64       `json:"price"`
	Quantity      int64       `json:"quantity"`
}

type SubmitOrderResult struct {
	Order  domain.Order
	Trades []domain.Trade
}

func (request SubmitOrderRequest) Validate() error {
	if request.ParticipantId <= 0 {
		return domain.ErrInvalidParticipantID
	}

	if strings.TrimSpace(request.Symbol) == "" {
		return domain.ErrEmptySymbol
	}

	if request.Side != domain.SideBuy &&
		request.Side != domain.SideSell {
		return domain.ErrInvalidSide
	}

	if request.Price <= 0 {
		return domain.ErrInvalidPrice
	}

	if request.Quantity <= 0 {
		return domain.ErrInvalidQty
	}

	return nil
}

