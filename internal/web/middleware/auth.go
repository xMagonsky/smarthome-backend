package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m *MiddlewareManager) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := m.auth.ValidateTokenJWT(c, c.GetHeader("Authorization"))
		if err != nil {
			println("Authentication error:", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)

		c.Next()
	}
}
