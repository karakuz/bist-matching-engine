package main

import (
	"context"
	"fmt"
	"log"
	stdhttp "net/http"
	"os"
	"time"

	"bist-matching-engine/internal/app"
	httptransport "bist-matching-engine/internal/http"
	"bist-matching-engine/internal/storage"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           BIST Matching Engine API
// @version         1.0
// @description     HTTP and WebSocket API for the BIST matching engine. Prices are integer kuruş values.
// @BasePath        /

var orderQueueCapacity = 5000

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
		return fmt.Errorf("create postgres pool: %w", err)
	}
	defer pool.Close()

	wsServer := app.NewWebSocketServer()
	store := storage.NewPostgresStore(pool)

	engines, err := initializeEngines(ctx, store)
	if err != nil {
		return fmt.Errorf("initialize engines: %w", err)
	}

	workers, err := initializeWorkers(
		ctx,
		store,
		engines,
		wsServer,
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

	router := gin.New()

	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		latencyMilliseconds := float64(param.Latency) / float64(time.Millisecond)

		return fmt.Sprintf(
			"[GIN] %s | %3d | %9.3f ms | %15s | %-7s %q\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			latencyMilliseconds,
			param.ClientIP,
			param.Method,
			param.Path,
		)
	}))

	router.Use(gin.Recovery())

	httptransport.RegisterRoutes(router, engines, workers)
	router.StaticFile("/swagger/doc.json", "docs/swagger.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/ws", gin.WrapF(wsServer.HandleConnection))

	router.POST("/publish", func(c *gin.Context) {
		topic := app.WsTopic(c.Query("topic"))
		symbol := c.Query("symbol")

		if !topic.IsValid() {
			c.JSON(stdhttp.StatusBadRequest, gin.H{"error": "invalid topic"})
			return
		}

		wsServer.Publish(
			c.Request.Context(),
			topic,
			symbol,
			[]byte(`{"price":123.45}`),
		)

		c.String(200, "published")
	})

	if err := router.Run(":8080"); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}

	return nil
}
