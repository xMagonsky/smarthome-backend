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

// EngineInterface defines the methods needed from the engine
type EngineInterface interface {
	RefreshRuleAssociations(ruleID string) error
	RemoveRuleAssociations(ruleID string) error
	TriggerRuleEvaluation(ruleID string)
}

type WebServer struct {
	router *gin.Engine
}

func NewWebServer(mqttClient MQTT.Client, dbConn *pgxpool.Pool, redisClient *redis.Client, JWTSecret string, engine EngineInterface, agentID string) *WebServer {
	router := gin.Default()

	authModule := auth.NewAuthModule(dbConn, redisClient, JWTSecret)
	middlewareManager := middleware.NewMiddlewareManager(dbConn, redisClient, authModule)
	// pumpService := services.NewPumpService(mqttClient)

	// api.RegisterTestRoutes(router, api.Dependencies{PumpService: pumpService})
	api.RegisterAuthRoutes(router, authModule, middlewareManager, agentID)
	api.RegisterDeviceRoutes(router, middlewareManager, dbConn, mqttClient)
	api.RegisterAutomationRoutes(router, middlewareManager, dbConn, engine)
	api.RegisterUserRoutes(router, middlewareManager, dbConn)

	// view.RegisterTestRoutes(router, view.Dependencies{DBConn: dbConn, RedisClient: redisClient})

	return &WebServer{router: router}
}

func (ws *WebServer) Start(addr string) {
	ws.router.Run(addr)
}
