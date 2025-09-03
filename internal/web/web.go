package web

import (
	"smarthome/internal/db"
	"smarthome/internal/services"
	"smarthome/internal/web/api"
	"smarthome/internal/web/view"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	redis "github.com/go-redis/redis/v8"
)

type WebServer struct {
	router *gin.Engine
}

func NewWebServer(mqttClient MQTT.Client, dbConn *db.DB, redisClient *redis.Client) *WebServer {
	router := gin.Default()

	pumpService := services.NewPumpService(mqttClient)

	api.RegisterTestRoutes(router, api.Dependencies{PumpService: pumpService})

	view.RegisterTestRoutes(router, view.Dependencies{DBConn: dbConn, RedisClient: redisClient})

	return &WebServer{router: router}
}

func (ws *WebServer) Start(addr string) {
	ws.router.Run(addr)
}
