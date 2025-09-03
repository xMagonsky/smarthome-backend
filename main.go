package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"smarthome/internal/config"
	"smarthome/internal/db"
	"smarthome/internal/engine"
	"smarthome/internal/mqtt"
	"smarthome/internal/redis"
	"smarthome/internal/scheduler"
	"smarthome/internal/taskqueue"
	"smarthome/internal/utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func test() {
	mqttOptions := MQTT.NewClientOptions()
	mqttOptions.AddBroker("tcp://magonsky.scay.net:1883")
	mqttOptions.SetClientID("backend-mqtt-client")
	mqttClient := MQTT.NewClient(mqttOptions)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Unable to connect to MQTT broker: %v", token.Error())
	}
	defer mqttClient.Disconnect(0)

	// webServer := web.NewWebServer(mqttClient)
	// webServer.Start(":5069")
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logging
	utils.InitLogging(cfg.LogLevel)

	// Initialize database
	dbConn, err := db.NewDB(cfg.DBURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer dbConn.Close(context.Background())

	// Initialize Redis
	redisClient := redis.NewRedisClient(cfg.RedisAddr)

	// Initialize MQTT
	mqttClient, err := mqtt.NewMQTTClient(cfg.MQTTBroker, cfg.MQTTClientID)
	if err != nil {
		log.Fatalf("Failed to connect to MQTT: %v", err)
	}

	// Initialize task queue with global instances
	taskqueue.SetGlobalInstances(dbConn, redisClient, mqttClient)

	// Initialize task queue workers
	go taskqueue.StartWorkers(cfg.RedisAddr)

	// Initialize scheduler
	sched := scheduler.NewScheduler(dbConn)
	sched.Start()

	// Initialize engine
	eng := engine.NewEngine(mqttClient, redisClient, dbConn, sched)
	if err := eng.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	eng.Stop()
	sched.Stop()
	taskqueue.StopWorkers()
	log.Println("Shutdown complete")
}
