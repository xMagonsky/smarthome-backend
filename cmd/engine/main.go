package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"smarthome/internal/config"
	"smarthome/internal/db"
	"smarthome/internal/engine"
	"smarthome/internal/internet_bridge"
	"smarthome/internal/mqtt"
	"smarthome/internal/redis"
	"smarthome/internal/scheduler"
	"smarthome/internal/taskqueue"
	"smarthome/internal/web"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func main() {
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbConn, err := db.NewDB(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer dbConn.Close(context.Background())

	redisClient := redis.NewRedisClient(cfg.Redis.Addr)

	mqttClient, err := mqtt.NewMQTTClient(cfg.MQTT.Broker, cfg.MQTT.ClientID)
	if err != nil {
		log.Fatalf("Failed to connect to MQTT: %v", err)
	}

	taskqueue.SetGlobalInstances(dbConn, redisClient, mqttClient)

	go taskqueue.StartWorkers(cfg.Redis.Addr)

	sched := scheduler.NewScheduler(dbConn)
	sched.Start()

	// Initialize engine first
	eng := engine.NewEngine(mqttClient, redisClient, dbConn, sched)
	if err := eng.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Pass engine to web server so it can notify about rule changes
	webServer := web.NewWebServer(mqttClient, dbConn.Pool(), redisClient, cfg.JWT.Secret, eng, cfg.App.AgentID)
	go webServer.Start(fmt.Sprintf(":%d", cfg.App.Port))

	// Start mDNS server
	go startMDNSServer(cfg.MDNS.LocalName)

	// Start remote access bridge if enabled
	if cfg.RemoteAccess.Enabled {
		uniqueID := cfg.App.AgentID
		internet_bridge.Start(internet_bridge.Config{
			PublicWS:   cfg.RemoteAccess.PublicWS,
			LocalURL:   "127.0.0.1:" + fmt.Sprintf("%d", cfg.App.Port),
			ServerID:   uniqueID,
			RetryDelay: time.Duration(cfg.RemoteAccess.RetryDelaySecs) * time.Second,
		})
	} else {
		log.Println("Remote access bridge is disabled")
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

func startMDNSServer(localName string) {
	addr4, err := net.ResolveUDPAddr("udp4", mdns.DefaultAddressIPv4)
	if err != nil {
		log.Println("Failed to resolve UDP4 address for mDNS:", err)
		return
	}

	addr6, err := net.ResolveUDPAddr("udp6", mdns.DefaultAddressIPv6)
	if err != nil {
		log.Println("Failed to resolve UDP6 address for mDNS:", err)
		return
	}

	l4, err := net.ListenUDP("udp4", addr4)
	if err != nil {
		log.Println("Failed to listen on UDP4 for mDNS:", err)
		return
	}

	l6, err := net.ListenUDP("udp6", addr6)
	if err != nil {
		log.Println("Failed to listen on UDP6 for mDNS:", err)
		return
	}

	_, err = mdns.Server(ipv4.NewPacketConn(l4), ipv6.NewPacketConn(l6), &mdns.Config{
		LocalNames: []string{localName},
	})
	if err != nil {
		log.Println("Failed to start mDNS server:", err)
	}
}
