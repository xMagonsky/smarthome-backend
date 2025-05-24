package view

import "github.com/gin-gonic/gin"

type Dependencies struct {
}

func RegisterRoutes(router *gin.Engine, deps Dependencies) {
	// Define your routes here
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Test route is working",
		})
	})
}
