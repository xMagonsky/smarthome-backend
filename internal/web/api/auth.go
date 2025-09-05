package api

import (
	"smarthome/auth"
	"smarthome/internal/web/middleware"
	"smarthome/internal/web/models"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(router *gin.Engine, authModule *auth.AuthModule, middlewareManager *middleware.MiddlewareManager) {
	r := router.Group("/auth")
	{
		r.POST("/login", func(c *gin.Context) {
			var loginRequest models.LoginRequest
			if err := c.ShouldBindJSON(&loginRequest); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}
			token, err := authModule.LoginWithJWT(c, loginRequest.Username, loginRequest.Password)
			if err != nil {
				c.JSON(401, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"token": token})
		})
		r.POST("/register", func(c *gin.Context) {
			var registerRequest models.RegisterRequest
			if err := c.ShouldBindJSON(&registerRequest); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}
			token, err := authModule.RegisterWithJWT(c, registerRequest.Username, registerRequest.Password, registerRequest.Email)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(201, gin.H{"token": token})
		})
	}
}
