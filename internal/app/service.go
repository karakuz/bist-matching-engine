package app

import (
	"context"
	"time"
	"errors"
	"strings"
	"fmt"

	"github.com/google/uuid"

	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"
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

func SubmitOrder(
	ctx context.Context,
	store *storage.PostgresStore,
	engine *matching.Engine,
	req SubmitOrderRequest,
) (SubmitOrderResult, error) {
	if err := req.Validate(); err != nil {
		return SubmitOrderResult{}, fmt.Errorf(
			"%w: %w",
			ErrInvalidOrder,
			err,
		)
	}

	symbol, err := store.GetSymbol(ctx, req.Symbol)
	if err != nil {
		return SubmitOrderResult{}, err
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		req.ParticipantId,
		symbol,
		engine.SessionDate(),
		req.Side,
		req.Price,
		req.Quantity,
		time.Now().UTC(),
	)
	if err != nil {
		return SubmitOrderResult{}, fmt.Errorf(
			"%w: %w",
			ErrInvalidOrder,
			err,
		)
	}

	if err := store.InsertOrder(ctx, order); err != nil {
		return SubmitOrderResult{}, fmt.Errorf(
			"insert order: %w",
			err,
		)
	}

	matchResult, err := engine.Submit(&order)
	if err != nil {
		return SubmitOrderResult{}, err
	}

	ordersToUpdate := append(
		[]domain.Order{*matchResult.IncomingOrder},
		matchResult.RestingOrderUpdates...,
	)

	if err := store.UpdateOrders(ctx, ordersToUpdate); err != nil {
		return SubmitOrderResult{}, err
	}

	if err := store.InsertTrades(ctx, matchResult.Trades); err != nil {
		return SubmitOrderResult{}, err
	}

	return SubmitOrderResult{
		Order:  *matchResult.IncomingOrder,
		Trades: matchResult.Trades,
	}, nil
}
