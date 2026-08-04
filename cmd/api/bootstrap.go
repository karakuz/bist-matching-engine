package main

import (
	"context"
	"fmt"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"
	"bist-matching-engine/internal/app"
)

func initializeEngines(ctx context.Context,store *storage.PostgresStore) (map[string]*matching.Engine, error) {
	initializations, err := store.GetBookInitializations(ctx)
	if err != nil {
		return nil, fmt.Errorf("get book initializations: %w",err)
	}

	if len(initializations) == 0 {
		return nil, fmt.Errorf("no book initializations found")
	}

	engines := make(map[string]*matching.Engine,len(initializations))

	for _, initialization := range initializations {
		orderBook := book.NewBook(initialization.Symbol,initialization.SessionDate,initialization.OpeningPrice)

		restingOrders, err :=
			store.GetRestingOrdersForSession(ctx, initialization.Symbol, initialization.SessionDate)
		if err != nil {
			return nil, fmt.Errorf("load resting orders for %s: %w",initialization.Symbol.Code,err)
		}

		if err := orderBook.Add(restingOrders...); err != nil {
			return nil, fmt.Errorf("restore order book for %s: %w",initialization.Symbol.Code,err)
		}

		engines[initialization.Symbol.Code] = matching.NewEngine(orderBook)
	}

	return engines, nil
}

func initializeWorkers(ctx context.Context, store *storage.PostgresStore, engines map[string]*matching.Engine, queueCapacity int) (map[string]*app.OrderWorker, error) {
	workers := make(
		map[string]*app.OrderWorker,
		len(engines),
	)

	for symbolCode, engine := range engines {
		symbol, err := store.GetSymbol(ctx, symbolCode)
		if err != nil {
			return nil, fmt.Errorf("get symbol %s: %w", symbolCode, err)
		}

		lastSequence, err := store.GetLastOrderSequence(ctx, symbolCode, engine.SessionDate())
		if err != nil {
			return nil, fmt.Errorf("get last sequence for %s: %w", symbolCode, err)
		}

		worker, err := app.NewOrderWorker(
			store,
			engine,
			symbol,
			lastSequence,
			queueCapacity,
		)
		if err != nil {
			return nil, fmt.Errorf("create worker for %s: %w", symbolCode, err)
		}

		workers[symbolCode] = worker
	}

	return workers, nil
}