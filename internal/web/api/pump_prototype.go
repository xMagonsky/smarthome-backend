package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
}

func RegisterTestRoutes(router *gin.Engine, deps Dependencies) {
	r := router.Group("")
	{
		r.GET("/api/pumps/main", getPumpStatusHandler)
	}
}

func getPumpStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "on", "message": "Pump is running"})
}
