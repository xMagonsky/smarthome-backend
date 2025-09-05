package web

import (
	"smarthome/auth"
	"smarthome/internal/web/api"
	"smarthome/internal/web/middleware"

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

	authModule := auth.NewAuthModule(dbConn, redisClient, JWTSecret)
	middlewareManager := middleware.NewMiddlewareManager(dbConn, redisClient, authModule)
	// pumpService := services.NewPumpService(mqttClient)

	// api.RegisterTestRoutes(router, api.Dependencies{PumpService: pumpService})
	api.RegisterAuthRoutes(router, authModule, middlewareManager)
	api.RegisterDeviceRoutes(router, middlewareManager, dbConn)
	api.RegisterAutomationRoutes(router, middlewareManager, dbConn)

	// view.RegisterTestRoutes(router, view.Dependencies{DBConn: dbConn, RedisClient: redisClient})

	return &WebServer{router: router}
}

func (ws *WebServer) Start(addr string) {
	ws.router.Run(addr)
}
