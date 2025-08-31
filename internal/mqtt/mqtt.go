package mqtt

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// NewMQTTClient creates an MQTT client
func NewMQTTClient(broker, clientID string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return client, nil
}

// Expand with more methods like Publish, Subscribe if needed
