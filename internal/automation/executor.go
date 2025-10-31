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
