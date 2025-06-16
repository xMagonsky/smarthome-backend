package services

import (
	"fmt"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type PumpService struct {
	MQTTClient MQTT.Client
}

func NewPumpService(client MQTT.Client) *PumpService {
	return &PumpService{MQTTClient: client}
}

func (ps *PumpService) GetStatus() (string, error) {
	statusCh := make(chan string)
	topic := "oczko/status"

	var receivedStatus string
	var handler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
		receivedStatus = string(msg.Payload())
		statusCh <- receivedStatus
	}

	token := ps.MQTTClient.Subscribe(topic, 0, handler)
	if token.Wait() && token.Error() != nil {
		return "", fmt.Errorf("failed to subscribe: %w", token.Error())
	}
	defer ps.MQTTClient.Unsubscribe(topic)

	// Wait for a message or timeout
	select {
	case status := <-statusCh:
		return status, nil
	case <-time.After(5 * time.Second): // 5 seconds timeout
		return "", fmt.Errorf("timeout waiting for status message")
	}
}
