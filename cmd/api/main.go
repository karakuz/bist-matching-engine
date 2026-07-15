package main

import (
	"context"
	"log"

	"bist-matching-engine/internal/book"
	httptransport "bist-matching-engine/internal/http"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	pool, err := storage.NewPostgresPoolFromEnv(ctx)
	if err != nil {
		log.Fatalf("failed to create postgres pool: %v", err)
	}
	defer pool.Close()

	store := storage.NewPostgresStore(pool)

	initializations, err := store.GetBookInitializations(ctx)
	if err != nil {
		log.Fatalf("could not fetch book initializations, err: %v", err)
	}

	engines := make(map[string]*matching.Engine, len(initializations))

	for _, initialization := range initializations {
		orderBook := book.NewBook(
			initialization.Symbol,
			initialization.SessionDate,
			initialization.OpeningPrice,
		)

		restingOrders, err := store.GetRestingOrdersForSession(
			ctx,
			initialization.Symbol,
			initialization.SessionDate,
		)
		if err != nil {
			log.Fatalf(
				"could not load resting orders for %s: %v",
				initialization.Symbol.Code,
				err,
			)
		}

		if err := orderBook.Add(restingOrders...); err != nil {
			log.Fatalf(
				"could not restore order book for %s: %v",
				initialization.Symbol.Code,
				err,
			)
		}

		var code string = initialization.Symbol.Code
		engines[code] = matching.NewEngine(orderBook)
	}

	router := gin.Default()
	httptransport.RegisterRoutes(router, store, engines)

	log.Fatal(router.Run(":8080"))
}
