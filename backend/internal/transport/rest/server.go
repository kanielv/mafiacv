package rest

import (
	"github.com/gin-gonic/gin"
	"github.com/kanielv/mafiacv/backend/internal/transport/ws"
)

func NewRouter(hub *ws.Hub) *gin.Engine {
	router := gin.Default()

	// CORS middleware for frontend
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/", TestHandler)
	router.GET("/ws", ws.ServeWS(hub))

	return router
}

func TestHandler(c *gin.Context) {
	c.String(200, "Hello from TestHandler!")
}
