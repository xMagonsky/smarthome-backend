package web

import (
	"smarthome/internal/web/view"

	"github.com/gin-gonic/gin"
)

type WebServer struct {
	router *gin.Engine
}

func NewWebServer() *WebServer {
	router := gin.Default()

	view.RegisterRoutes(router, view.Dependencies{})

	return &WebServer{
		router: router,
	}
}

func (ws *WebServer) Start(addr string) {
	ws.router.Run(addr)
}
