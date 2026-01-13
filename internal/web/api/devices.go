package api

import (
	"encoding/json"
	"fmt"

	"smarthome/internal/models"
	"smarthome/internal/web/middleware"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterDeviceRoutes(r *gin.Engine, middleware *middleware.MiddlewareManager, dbConn *pgxpool.Pool, mqttClient mqtt.Client) {
	devices := r.Group("/devices")
	devices.Use(middleware.RequireAuth())
	{
		devices.GET("/", func(c *gin.Context) {
			userID := c.GetString("user_id")
			println("User ID from context:", userID)
			rows, err := dbConn.Query(c, "SELECT id, name, type, state, mqtt_topic, accepted, owner_id FROM devices WHERE owner_id=$1 AND accepted=true", userID)
			if err != nil {
				println("Error fetching devices:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to fetch devices"})
				return
			}
			defer rows.Close()

			devices := []models.Device{}
			for rows.Next() {
				var device models.Device
				if err := rows.Scan(&device.ID, &device.Name, &device.Type, &device.State, &device.MQTTTopic, &device.Accepted, &device.OwnerID); err != nil {
					c.JSON(500, gin.H{"error": "Failed to scan device"})
					return
				}
				devices = append(devices, device)
			}

			c.JSON(200, devices)
		})

		devices.GET("/pending", func(c *gin.Context) {
			rows, err := dbConn.Query(c, "SELECT id, name, type, state, mqtt_topic, accepted, owner_id FROM devices WHERE accepted=false")
			if err != nil {
				println("Error fetching pending devices:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to fetch pending devices"})
				return
			}
			defer rows.Close()

			devices := []models.Device{}
			for rows.Next() {
				var device models.Device
				if err := rows.Scan(&device.ID, &device.Name, &device.Type, &device.State, &device.MQTTTopic, &device.Accepted, &device.OwnerID); err != nil {
					c.JSON(500, gin.H{"error": "Failed to scan device"})
					return
				}
				devices = append(devices, device)
			}

			c.JSON(200, devices)
		})

		devices.POST("/:id/accept", func(c *gin.Context) {
			deviceID := c.Param("id")
			userID := c.GetString("user_id")

			commandTag, err := dbConn.Exec(c, "UPDATE devices SET accepted=true, owner_id=$1 WHERE id=$2", userID, deviceID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to accept device"})
				return
			}
			if commandTag.RowsAffected() == 0 {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}
			c.JSON(200, gin.H{"status": "Device accepted successfully"})
		})

		devices.PATCH("/:id/setowner", func(c *gin.Context) {
			deviceID := c.Param("id")
			newOwnerID := c.GetString("user_id")

			commandTag, err := dbConn.Exec(c, "UPDATE devices SET owner_id=$1 WHERE id=$2", newOwnerID, deviceID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to update device owner"})
				return
			}
			if commandTag.RowsAffected() == 0 {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}
			c.JSON(200, gin.H{"status": "Owner updated successfully"})
		})

		devices.POST("/:id/command", func(c *gin.Context) {
			deviceID := c.Param("id")
			userID := c.GetString("user_id")

			// Verify device ownership and acceptance
			var ownerID *string
			var accepted bool
			err := dbConn.QueryRow(c, "SELECT owner_id, accepted FROM devices WHERE id=$1", deviceID).Scan(&ownerID, &accepted)
			if err != nil {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}
			if !accepted {
				c.JSON(403, gin.H{"error": "Device not accepted"})
				return
			}
			if ownerID == nil || *ownerID != userID {
				c.JSON(403, gin.H{"error": "Unauthorized: You don't own this device"})
				return
			}

			// Parse command parameters from request body
			var commandParams map[string]interface{}
			if err := c.ShouldBindJSON(&commandParams); err != nil {
				c.JSON(400, gin.H{"error": "Invalid command parameters"})
				return
			}

			// Publish command to MQTT
			if mqttClient != nil {
				payload, err := json.Marshal(commandParams)
				if err != nil {
					c.JSON(500, gin.H{"error": "Failed to encode command"})
					return
				}

				topic := fmt.Sprintf("devices/%s/commands", deviceID)
				token := mqttClient.Publish(topic, 1, false, payload)
				token.Wait()

				if token.Error() != nil {
					c.JSON(500, gin.H{"error": "Failed to publish command"})
					return
				}

				c.JSON(200, gin.H{
					"status":  "Command sent successfully",
					"topic":   topic,
					"command": commandParams,
				})
			} else {
				c.JSON(500, gin.H{"error": "MQTT client not available"})
			}
		})

		devices.PATCH("/:id/name", func(c *gin.Context) {
			deviceID := c.Param("id")
			userID := c.GetString("user_id")

			// Verify device ownership
			var ownerID *string
			err := dbConn.QueryRow(c, "SELECT owner_id FROM devices WHERE id=$1", deviceID).Scan(&ownerID)
			if err != nil {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}
			if ownerID == nil || *ownerID != userID {
				c.JSON(403, gin.H{"error": "Unauthorized: You don't own this device"})
				return
			}

			// Parse new name from request body
			var requestBody struct {
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&requestBody); err != nil {
				c.JSON(400, gin.H{"error": "Invalid request: name is required"})
				return
			}

			// Update device name
			commandTag, err := dbConn.Exec(c, "UPDATE devices SET name=$1 WHERE id=$2", requestBody.Name, deviceID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to update device name"})
				return
			}
			if commandTag.RowsAffected() == 0 {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}

			c.JSON(200, gin.H{
				"status": "Device name updated successfully",
				"name":   requestBody.Name,
			})
		})

		devices.DELETE("/:id", func(c *gin.Context) {
			deviceID := c.Param("id")
			userID := c.GetString("user_id")

			// Verify device ownership
			var ownerID *string
			err := dbConn.QueryRow(c, "SELECT owner_id FROM devices WHERE id=$1", deviceID).Scan(&ownerID)
			if err != nil {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}
			if ownerID == nil || *ownerID != userID {
				c.JSON(403, gin.H{"error": "Unauthorized: You don't own this device"})
				return
			}

			// Delete the device
			commandTag, err := dbConn.Exec(c, "DELETE FROM devices WHERE id=$1", deviceID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to delete device"})
				return
			}
			if commandTag.RowsAffected() == 0 {
				c.JSON(404, gin.H{"error": "Device not found"})
				return
			}

			c.JSON(200, gin.H{
				"status": "Device deleted successfully",
			})
		})
	}
}
