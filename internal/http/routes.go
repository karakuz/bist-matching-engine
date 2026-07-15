package http

import (
	stdhttp "net/http"

	"bist-matching-engine/internal/matching"
	"bist-matching-engine/internal/storage"

	"github.com/gin-gonic/gin"
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