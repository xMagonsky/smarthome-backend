package engine

import (
	"encoding/json"
	"fmt"

	"smarthome/internal/automation"
	"smarthome/internal/models"
)

// ExecuteActions executes rule actions (wrapper for Engine)
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
					// Removed utils.SendNotification, logging instead
					fmt.Printf("ENGINE: Sending notification: %s\n", msg)
				}
			}
		}
		// Add logging or metrics here for expansion
	}
	//go e.db.LogAction(context.Background() /* ruleID */, "" /* deviceID */, "" /* state */, nil) // Placeholder
}

// ExecuteActionsStatic is a convenience wrapper for automation.ExecuteActions
func (e *Engine) ExecuteActionsStatic(actionsRaw json.RawMessage) {
	automation.ExecuteActions(e.mqttClient, actionsRaw)
}

// Expand with more action types (e.g., API calls, integrations)
