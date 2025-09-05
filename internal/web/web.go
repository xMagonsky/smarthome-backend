package web

import (
	"smarthome/internal/web/api"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type WebServer struct {
	router *gin.Engine
}

func NewWebServer(mqttClient MQTT.Client, dbConn *pgxpool.Pool, redisClient *redis.Client, JWTSecret string) *WebServer {
	router := gin.Default()

	// pumpService := services.NewPumpService(mqttClient)

	// api.RegisterTestRoutes(router, api.Dependencies{PumpService: pumpService})

	// view.RegisterTestRoutes(router, view.Dependencies{DBConn: dbConn, RedisClient: redisClient})

	api.RegisterAuthRoutes(router, dbConn, redisClient, JWTSecret)

	return &WebServer{router: router}
}

func (ws *WebServer) Start(addr string) {
	ws.router.Run(addr)
}
