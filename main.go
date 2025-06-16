package main

import (
	"log"
	"smarthome/internal/web"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	mqttOptions := MQTT.NewClientOptions()
	mqttOptions.AddBroker("tcp://magonsky.scay.net:1883")
	mqttOptions.SetClientID("backend-mqtt-client")
	mqttClient := MQTT.NewClient(mqttOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Unable to connect to MQTT broker: %v", token.Error())
	}
	defer mqttClient.Disconnect(0)

	webServer := web.NewWebServer(mqttClient)
	webServer.Start(":5069")
}
