package api

import (
	"smarthome/auth"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func RegisterAuthRoutes(router *gin.Engine, dbConn *pgxpool.Pool, redisClient *redis.Client, JWTSecret string) {
	authModule := auth.NewAuthModule(dbConn, redisClient, JWTSecret)

	r := router.Group("/auth")
	{
		r.POST("/login", func(c *gin.Context) {
			var loginRequest LoginRequest
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
			var registerRequest RegisterRequest
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
