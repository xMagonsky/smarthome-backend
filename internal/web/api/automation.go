package api

import (
	"smarthome/internal/models"
	"smarthome/internal/web/middleware"
	webModels "smarthome/internal/web/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterAutomationRoutes(r *gin.Engine, middleware *middleware.MiddlewareManager, dbConn *pgxpool.Pool) {
	automations := r.Group("/automations")
	automations.Use(middleware.RequireAuth())
	{
		automations.GET("/rules", func(c *gin.Context) {
			userID := c.GetString("user_id")
			println("User ID from context:", userID)
			rows, err := dbConn.Query(c, "SELECT id, name, conditions, actions, enabled, owner_id FROM rules WHERE owner_id=$1", userID)
			if err != nil {
				println("Error fetching rules:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to fetch rules"})
				return
			}
			defer rows.Close()

			automations := []models.Rule{}
			for rows.Next() {
				var a models.Rule
				if err := rows.Scan(&a.ID, &a.Name, &a.Conditions, &a.Actions, &a.Enabled, &a.OwnerID); err != nil {
					println("Error scanning rule:", err.Error())
					continue
				}
				automations = append(automations, a)
			}
			c.JSON(200, automations)
		})

		automations.POST("/rules", func(c *gin.Context) {
			userID := c.GetString("user_id")
			var newRuleReq webModels.AddRuleRequest
			if err := c.ShouldBindJSON(&newRuleReq); err != nil {
				println("Error binding JSON:", err.Error())
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}
			_, err := dbConn.Exec(c, "INSERT INTO rules (name, conditions, actions, enabled, owner_id) VALUES ($1, $2, $3, $4, $5)",
				newRuleReq.Name, newRuleReq.Conditions, newRuleReq.Actions, newRuleReq.Enabled, userID)
			if err != nil {
				println("Error creating rule:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to create rule"})
				return
			}
			c.JSON(201, gin.H{"status": "Rule created successfully"})
		})

		automations.DELETE("/rules/:id", func(c *gin.Context) {
			userID := c.GetString("user_id")
			ruleID := c.Param("id")
			_, err := dbConn.Exec(c, "DELETE FROM rules WHERE id=$1 AND owner_id=$2", ruleID, userID)
			if err != nil {
				println("Error deleting rule:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to delete rule"})
				return
			}
			c.JSON(200, gin.H{"status": "Rule deleted successfully"})
		})

		automations.PATCH("/rules/:id", func(c *gin.Context) {
			userID := c.GetString("user_id")
			ruleID := c.Param("id")
			var updateRuleReq webModels.AddRuleRequest
			if err := c.ShouldBindJSON(&updateRuleReq); err != nil {
				println("Error binding JSON:", err.Error())
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}
			_, err := dbConn.Exec(c, "UPDATE rules SET name=$1, conditions=$2, actions=$3, enabled=$4 WHERE id=$5 AND owner_id=$6",
				updateRuleReq.Name, updateRuleReq.Conditions, updateRuleReq.Actions, updateRuleReq.Enabled, ruleID, userID)
			if err != nil {
				println("Error updating rule:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to update rule"})
				return
			}
			c.JSON(200, gin.H{"status": "Rule updated successfully"})
		})
	}
}
