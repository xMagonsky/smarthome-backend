package engine

import (
	"context"
	"encoding/json"
	"fmt"

	"smarthome/internal/models"
	"smarthome/internal/utils"
)

// ExecuteActions executes rule actions
func (e *Engine) ExecuteActions(actions []models.Action) {
	for _, action := range actions {
		if action.DeviceID != "" {
			payload, _ := json.Marshal(action.Params)
			e.mqttClient.Publish(fmt.Sprintf("devices/%s/commands", action.DeviceID), 1, false, payload)
		}
		if action.Action == "send_email" {
			var paramsMap map[string]interface{}
			if err := json.Unmarshal(action.Params, &paramsMap); err == nil {
				if msg, ok := paramsMap["message"].(string); ok {
					go utils.SendNotification(msg)
				}
			}
		}
		// Add logging or metrics here for expansion
	}
	go e.db.LogAction(context.Background() /* ruleID */, "" /* deviceID */, "" /* state */, nil) // Placeholder
}

// Expand with more action types (e.g., API calls, integrations)
