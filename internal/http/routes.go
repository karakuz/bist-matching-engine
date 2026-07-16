package http

import (
	"errors"
	"log"
	stdhttp "net/http"

	"bist-matching-engine/internal/book"
	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"

	"github.com/gin-gonic/gin"

	"fmt"
	"strconv"
	"strings"
)

func RegisterRoutes(
	router *gin.Engine,
	store *storage.PostgresStore,
	engine map[string]*matching.Engine,
) {
	router.GET("/alivez", func(c *gin.Context) {
		c.JSON(stdhttp.StatusOK, []gin.H{
			{"Test": "123"},
		})
		return
	})

	router.GET("/orderbook/:symbol/:levels", func(c *gin.Context) {
		const maxSnapshotLevels = 100

		symbol := strings.ToUpper(c.Param("symbol"))
		levelsStr := c.Param("levels")

		orderBook := engine[symbol]
		if orderBook == nil {
			c.JSON(stdhttp.StatusNotFound, gin.H{
				"message": fmt.Sprintf("order book for '%s' not found", symbol),
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

		snapshot, err := orderBook.Snapshot(int64(levels))
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

	/* router.POST("/orders", func(c *gin.Context) {
		var req app.SubmitOrderRequest

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{
				"error": "invalid request body",
			})
			return
		}

		result, err := app.SubmitOrder(c.Request.Context(), store, engine, req)
		if err != nil {
			c.JSON(stdhttp.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(stdhttp.StatusCreated, result)
	}) */
}
