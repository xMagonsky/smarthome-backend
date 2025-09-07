package api

import (
	"log"
	"smarthome/internal/web/middleware"
	"smarthome/internal/web/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterUserRoutes(r *gin.Engine, middleware *middleware.MiddlewareManager, dbConn *pgxpool.Pool) {
	users := r.Group("/users")
	users.Use(middleware.RequireAuth())
	{
		users.GET("/me", func(c *gin.Context) {
			userID := c.GetString("user_id")
			var user models.User
			err := dbConn.QueryRow(c, "SELECT id, email, username FROM users WHERE id=$1", userID).Scan(&user.ID, &user.Email, &user.Name)
			if err != nil {
				log.Printf("API: Failed to fetch user data: %v", err)
				c.JSON(404, gin.H{"error": "User not found"})
				return
			}
			c.JSON(200, user)
		})
	}
}
