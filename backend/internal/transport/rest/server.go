package rest

import "github.com/gin-gonic/gin"

func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/", TestHandler)
	return router
}

func TestHandler(c *gin.Context) {
	c.String(200, "Hello from TestHandler!")
}
