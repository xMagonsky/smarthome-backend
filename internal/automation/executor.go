package automation

import (
	"encoding/json"
	"fmt"
	"log"

	"smarthome/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ExecuteActions executes rule actions
func ExecuteActions(mqttClient mqtt.Client, actionsRaw json.RawMessage) {
	log.Printf("AUTOMATION: Starting action execution")
	var actions []models.Action
	if err := json.Unmarshal(actionsRaw, &actions); err != nil {
		log.Printf("AUTOMATION: Failed to unmarshal actions: %v", err)
		return
	}
	log.Printf("AUTOMATION: Executing %d actions", len(actions))

	for _, action := range actions {
		if action.DeviceID != "" && mqttClient != nil {
			payload, _ := json.Marshal(action.Params)
			topic := fmt.Sprintf("devices/%s/commands", action.DeviceID)
			log.Printf("AUTOMATION: Publishing MQTT command to %s: %s", topic, string(payload))
			mqttClient.Publish(topic, 1, false, payload)
		}
		if action.Action == "send_email" {
			var paramsMap map[string]interface{}
			if err := json.Unmarshal(action.Params, &paramsMap); err == nil {
				if msg, ok := paramsMap["message"].(string); ok {
					log.Printf("AUTOMATION: Sending notification: %s", msg)
				}
			}
		}
	}
	log.Printf("AUTOMATION: Action execution completed")
}

// ExecuteResolvedActions executes conflict-resolved actions
// resolvedActions is a map of deviceID -> params (attribute -> value)
func ExecuteResolvedActions(mqttClient mqtt.Client, resolvedActions map[string]map[string]interface{}) {
	log.Printf("AUTOMATION: Starting resolved action execution for %d devices", len(resolvedActions))

	if mqttClient == nil {
		log.Printf("AUTOMATION: MQTT client not available")
		return
	}

	for deviceID, params := range resolvedActions {
		if len(params) > 0 {
			payload, err := json.Marshal(params)
			if err != nil {
				log.Printf("AUTOMATION: Failed to marshal params for device %s: %v", deviceID, err)
				continue
			}

			topic := fmt.Sprintf("devices/%s/commands", deviceID)
			log.Printf("AUTOMATION: Publishing resolved MQTT command to %s: %s", topic, string(payload))
			mqttClient.Publish(topic, 1, false, payload)
		}
	}

	log.Printf("AUTOMATION: Resolved action execution completed")
}
