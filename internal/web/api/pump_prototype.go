package api

import (
	"net/http"

	"smarthome/internal/services"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	PumpService *services.PumpService
}

func RegisterTestRoutes(router *gin.Engine, deps Dependencies) {
	r := router.Group("")
	{
		r.GET("/api/pumps/main", func(c *gin.Context) {
			status, err := deps.PumpService.GetStatus()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"status": status})
		})
	}
}
