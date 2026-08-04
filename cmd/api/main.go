package main

import (
	"context"
	"log"
	"os"
	"fmt"

	httptransport "bist-matching-engine/internal/http"
	"bist-matching-engine/internal/storage"

	"github.com/gin-gonic/gin"
)

const orderQueueCapacity = 100

func main() {
	if err := run(); err != nil {
		log.Printf("application failed: %v", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	pool, err := storage.NewPostgresPoolFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w",err)
	}
	defer pool.Close()

	store := storage.NewPostgresStore(pool)

	engines, err := initializeEngines(ctx, store)
	if err != nil {
		return fmt.Errorf("initialize engines: %w",err)
	}

	workers, err := initializeWorkers(
		ctx,
		store,
		engines,
		orderQueueCapacity,
	)
	if err != nil {
		return fmt.Errorf("initialize workers: %w", err)
	}

	defer func() {
		for _, worker := range workers {
			worker.Stop()
		}
	}()

	router := gin.Default()
	httptransport.RegisterRoutes(router, engines, workers)

	if err := router.Run(":8080"); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}

	return nil
}