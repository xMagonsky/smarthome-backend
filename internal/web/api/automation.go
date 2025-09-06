package api

import (
	"log"
	"smarthome/internal/models"
	"smarthome/internal/web/middleware"
	webModels "smarthome/internal/web/models"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EngineInterface defines the methods needed from the engine
type EngineInterface interface {
	RefreshRuleAssociations(ruleID string) error
	RemoveRuleAssociations(ruleID string) error
	TriggerRuleEvaluation(ruleID string)
}

func RegisterAutomationRoutes(r *gin.Engine, middleware *middleware.MiddlewareManager, dbConn *pgxpool.Pool, engine EngineInterface) {
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

			var createdRule models.Rule
			err = dbConn.QueryRow(c, "SELECT id, name, conditions, actions, enabled, owner_id FROM rules WHERE name=$1 AND owner_id=$2 ORDER BY id DESC LIMIT 1",
				newRuleReq.Name, userID).Scan(&createdRule.ID, &createdRule.Name, &createdRule.Conditions, &createdRule.Actions, &createdRule.Enabled, &createdRule.OwnerID)
			if err != nil {
				println("Error fetching created rule:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to fetch created rule"})
				return
			}

			// Refresh engine associations for the new rule
			if err := engine.RefreshRuleAssociations(createdRule.ID); err != nil {
				log.Printf("Error refreshing rule associations for rule %s: %v", createdRule.ID, err)
				// Don't fail the request, just log the error
			} else {
				log.Printf("Successfully refreshed rule associations for new rule %s", createdRule.ID)
				// Trigger immediate evaluation of the new rule if it's enabled
				if createdRule.Enabled {
					engine.TriggerRuleEvaluation(createdRule.ID)
					log.Printf("Triggered immediate evaluation for new rule %s", createdRule.ID)
				}
			}

			c.JSON(201, createdRule)
		})

		automations.DELETE("/rules/:id", func(c *gin.Context) {
			userID := c.GetString("user_id")
			ruleID := c.Param("id")

			// Remove engine associations before deleting the rule
			if err := engine.RemoveRuleAssociations(ruleID); err != nil {
				log.Printf("Error removing rule associations for rule %s: %v", ruleID, err)
				// Don't fail the request, just log the error
			} else {
				log.Printf("Successfully removed rule associations for rule %s", ruleID)
			}

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
			var updateRuleReq webModels.UpdateRuleRequest
			if err := c.ShouldBindJSON(&updateRuleReq); err != nil {
				println("Error binding JSON:", err.Error())
				c.JSON(400, gin.H{"error": "Invalid request"})
				return
			}

			// First get the existing rule
			var existingRule models.Rule
			row := dbConn.QueryRow(c, "SELECT id, name, conditions, actions, enabled, owner_id FROM rules WHERE id=$1 AND owner_id=$2", ruleID, userID)
			if err := row.Scan(&existingRule.ID, &existingRule.Name, &existingRule.Conditions, &existingRule.Actions, &existingRule.Enabled, &existingRule.OwnerID); err != nil {
				println("Error fetching existing rule:", err.Error())
				c.JSON(404, gin.H{"error": "Rule not found"})
				return
			}

			// Update only the fields that were provided
			if updateRuleReq.Name != nil {
				existingRule.Name = *updateRuleReq.Name
			}
			if updateRuleReq.Conditions != nil {
				existingRule.Conditions = *updateRuleReq.Conditions
			}
			if updateRuleReq.Actions != nil {
				existingRule.Actions = *updateRuleReq.Actions
			}
			if updateRuleReq.Enabled != nil {
				existingRule.Enabled = *updateRuleReq.Enabled
			}

			_, err := dbConn.Exec(c, "UPDATE rules SET name=$1, conditions=$2, actions=$3, enabled=$4 WHERE id=$5 AND owner_id=$6",
				existingRule.Name, existingRule.Conditions, existingRule.Actions, existingRule.Enabled, existingRule.ID, existingRule.OwnerID)
			if err != nil {
				println("Error updating rule:", err.Error())
				c.JSON(500, gin.H{"error": "Failed to update rule"})
				return
			}

			// Refresh engine associations for the updated rule
			if err := engine.RefreshRuleAssociations(existingRule.ID); err != nil {
				log.Printf("Error refreshing rule associations for rule %s: %v", existingRule.ID, err)
				// Don't fail the request, just log the error
			} else {
				log.Printf("Successfully refreshed rule associations for updated rule %s", existingRule.ID)
				// Trigger immediate evaluation of the updated rule if it's enabled
				if existingRule.Enabled {
					engine.TriggerRuleEvaluation(existingRule.ID)
					log.Printf("Triggered immediate evaluation for updated rule %s", existingRule.ID)
				}
			}

			c.JSON(200, existingRule)
		})
	}
}
