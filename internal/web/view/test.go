package view

import (
	"html/template"
	"net/http"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
}

func RegisterRoutes(router *gin.Engine, deps Dependencies) {

	router.SetFuncMap(template.FuncMap{
		"title": func(s string) string {
			return strings.ToUpper(s)
		},
	})
	router.LoadHTMLGlob("internal/web/view/templates/*")

	router.GET("/test", func(c *gin.Context) {
		data := gin.H{
			"Message":     "This is a dynamic message!",
			"ShowDetails": true,
			"Items":       []string{"Item 1", "Item 2", "Item 3"},
			"Name":        "john doe",
		}
		c.HTML(http.StatusOK, "test.html", data)
	})

	router.POST("/test", func(c *gin.Context) {
		action := c.PostForm("action")

		opts := mqtt.NewClientOptions().AddBroker("tcp://magonsky.scay.net:1883")
		opts.SetClientID("smarthome-backend")
		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			c.String(http.StatusInternalServerError, "MQTT connection error: %v", token.Error())
			return
		}
		defer client.Disconnect(250)

		if action == "ON" {
			client.Publish("test10/led", 0, false, "ON")
		} else if action == "OFF" {
			client.Publish("test10/led", 0, false, "OFF")
		}

		c.Redirect(http.StatusSeeOther, "/test")
	})
}
