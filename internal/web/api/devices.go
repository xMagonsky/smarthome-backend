package api

import (
	"smarthome/internal/models"
	"smarthome/internal/web/middleware"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterDeviceRoutes(r *gin.Engine, middleware *middleware.MiddlewareManager, dbConn *pgxpool.Pool) {
	devices := r.Group("/devices")
	devices.Use(middleware.RequireAuth())
	{
		devices.GET("/", func(c *gin.Context) {
			userID := c.GetString("user_id")
			println("User ID from context:", userID)
			rows, err := dbConn.Query(c, "SELECT id, name, type, state, mqtt_topic FROM devices WHERE owner_id=$1", userID)
			if err != nil {
				println("Error fetching devices:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to fetch devices"})
				return
			}
			defer rows.Close()

			var devices []models.Device
			for rows.Next() {
				var device models.Device
				if err := rows.Scan(&device.ID, &device.Name, &device.Type, &device.State, &device.MQTTTopic); err != nil {
					c.JSON(500, gin.H{"error": "Failed to scan device"})
					return
				}
				devices = append(devices, device)
			}

			c.JSON(200, devices)
		})
	}
}
