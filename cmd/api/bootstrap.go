package main

import (
	"context"
	"fmt"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"
)

func initializeEngines(
	ctx context.Context,
	store *storage.PostgresStore,
) (map[string]*matching.Engine, error) {
	initializations, err := store.GetBookInitializations(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"get book initializations: %w",
			err,
		)
	}

	if len(initializations) == 0 {
		return nil, fmt.Errorf("no book initializations found")
	}

	engines := make(
		map[string]*matching.Engine,
		len(initializations),
	)

	for _, initialization := range initializations {
		orderBook := book.NewBook(
			initialization.Symbol,
			initialization.SessionDate,
			initialization.OpeningPrice,
		)

		restingOrders, err :=
			store.GetRestingOrdersForSession(
				ctx,
				initialization.Symbol,
				initialization.SessionDate,
			)
		if err != nil {
			return nil, fmt.Errorf(
				"load resting orders for %s: %w",
				initialization.Symbol.Code,
				err,
			)
		}

		if err := orderBook.Add(restingOrders...); err != nil {
			return nil, fmt.Errorf(
				"restore order book for %s: %w",
				initialization.Symbol.Code,
				err,
			)
		}

		engines[initialization.Symbol.Code] =
			matching.NewEngine(orderBook)
	}

	return engines, nil
}