package mqtt

import (
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// NewClient initializes and returns a raw MQTT.Client
func NewClient(broker, clientID string) (MQTT.Client, error) {
	opts := MQTT.NewClientOptions().AddBroker(broker).SetClientID(clientID)
	c := MQTT.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}
	return c, nil
}
