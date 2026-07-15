package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"bist-matching-engine/internal/domain"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"
)

type SubmitOrderRequest struct {
	ParticipantId int64
	Symbol   string
	Side     domain.Side
	Price    int64
	Quantity int64
}

type SubmitOrderResult struct {
	Order  domain.Order
	Trades []domain.Trade
}

func SubmitOrder(
	ctx context.Context,
	store *storage.PostgresStore,
	engine *matching.Engine,
	req SubmitOrderRequest,
) (SubmitOrderResult, error) {
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
		return SubmitOrderResult{}, err
	}

	if err := store.InsertOrder(ctx, order); err != nil {
		return SubmitOrderResult{}, err
	}

	updatedOrder, trades, err := engine.Submit(&order)
	if err != nil {
		return SubmitOrderResult{}, err
	}

	if err := store.UpdateOrder(ctx, *updatedOrder); err != nil {
		return SubmitOrderResult{}, err
	}

	if err := store.InsertTrades(ctx, trades); err != nil {
		return SubmitOrderResult{}, err
	}

	return SubmitOrderResult{
		Order:  *updatedOrder,
		Trades: trades,
	}, nil
}