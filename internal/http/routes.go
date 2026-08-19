package http

import (
	"errors"
	"log"
	stdhttp "net/http"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/app"

	"github.com/gin-gonic/gin"

	"fmt"
	"strconv"
	"strings"
	"context"
)

func RegisterRoutes(
	router *gin.Engine,
	engine map[string]*matching.Engine,
	workers map[string]*app.OrderWorker,
) {
	router.GET("/alivez", func(c *gin.Context) {
		c.JSON(stdhttp.StatusOK, []gin.H{
			{"Test": "123"},
		})
	})

	router.GET("/engine/:symbol/snapshot/:levels", func(c *gin.Context) {
		const maxSnapshotLevels = 100

		symbol := strings.ToUpper(c.Param("symbol"))
		levelsStr := c.Param("levels")

		orderEngine  := engine[symbol]
		if orderEngine  == nil {
			c.JSON(stdhttp.StatusNotFound, gin.H{
				"message": fmt.Sprintf("order engine for '%s' not found", symbol),
			})
			return
		}

		levels, err := strconv.Atoi(levelsStr)
		//levelsStr should be converted into int
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{
				"message": "levels must be a positive integer",
			})
			return
		}

		snapshot, err := orderEngine.Snapshot(int64(levels))
		if err != nil {
			switch {
			case errors.Is(err, book.ErrSnapshotSizeNonPositive):
				c.JSON(stdhttp.StatusBadRequest, gin.H{
					"message": "levels must be a positive integer",
				})
			case errors.Is(err, book.ErrRequestedMoreLevelsThanMaxSnapshotSize):
				c.JSON(stdhttp.StatusBadRequest, gin.H{
					"message": fmt.Sprintf("levels must less than %d", book.MAX_SNAPSHOT_LEVELS),
				})
			default:
				log.Printf("order book snapshot failed: %v", err)

				c.JSON(stdhttp.StatusInternalServerError, gin.H{
					"message": "internal server error",
				})
			}

			return
		}

		c.JSON(stdhttp.StatusOK, snapshot)
	})

	router.POST("/orders", func(c *gin.Context) {
		var req app.SubmitOrderRequest

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{
				"error": "invalid request body",
			})
			return
		}

		req.Symbol = strings.ToUpper(req.Symbol)
		worker := workers[req.Symbol]
		if worker == nil {
			c.JSON(stdhttp.StatusNotFound, gin.H{
				"message": fmt.Sprintf("order worker for '%s' not found",req.Symbol),
			})
			return
		}

		err = worker.Submit(c.Request.Context(),req)
		if err != nil {
			switch {
			case errors.Is(err, app.ErrInvalidOrder):
				c.JSON(stdhttp.StatusBadRequest, gin.H{
					"message": err.Error(),
				})

			case errors.Is(err, app.ErrSubmissionQueueFull):
				c.JSON(stdhttp.StatusServiceUnavailable, gin.H{
					"message": "order submission queue is full",
				})

			case errors.Is(err, app.ErrWorkerStopped):
				c.JSON(stdhttp.StatusServiceUnavailable, gin.H{
					"message": "order submission is unavailable",
				})

			case errors.Is(err, context.Canceled):
				return

			default:
				log.Printf("submit order failed: %v", err)

				c.JSON(stdhttp.StatusInternalServerError, gin.H{
					"message": "internal server error",
				})
			}

			return
		}

		c.JSON(stdhttp.StatusAccepted, map[string]string{
			"message": "Order Queued",
		})
	})
}
