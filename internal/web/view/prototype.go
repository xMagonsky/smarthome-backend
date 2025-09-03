package view

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"smarthome/internal/db"
	"smarthome/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gin-gonic/gin"
	redis "github.com/go-redis/redis/v8"
)

type Dependencies struct {
	DBConn      *db.DB
	RedisClient *redis.Client
}

func RegisterTestRoutes(router *gin.Engine, deps Dependencies) {

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

		switch action {
		case "ON":
			client.Publish("test10/led", 0, false, "ON")
		case "OFF":
			client.Publish("test10/led", 0, false, "OFF")
		}

		c.Redirect(http.StatusSeeOther, "/test")
	})

	// Rule testing routes
	router.GET("/rules", func(c *gin.Context) {
		rules, err := deps.DBConn.GetAllRules(context.Background())
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.HTML(http.StatusOK, "rules.html", gin.H{
			"Rules": rules,
		})
	})

	router.GET("/rules/test/:id", func(c *gin.Context) {
		ruleID := c.Param("id")

		rule, err := deps.DBConn.GetRuleByID(context.Background(), ruleID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.HTML(http.StatusOK, "rule_test.html", gin.H{
			"Rule": rule,
		})
	})

	router.POST("/rules/test/:id", func(c *gin.Context) {
		ruleID := c.Param("id")

		rule, err := deps.DBConn.GetRuleByID(context.Background(), ruleID)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		// Get test data from form
		testData := make(map[string]interface{})
		for key, values := range c.Request.PostForm {
			if len(values) > 0 {
				// Try to parse as number first
				if val, err := strconv.ParseFloat(values[0], 64); err == nil {
					testData[key] = val
				} else if val, err := strconv.ParseBool(values[0]); err == nil {
					testData[key] = val
				} else {
					testData[key] = values[0]
				}
			}
		}

		// Convert to JSON for testing
		testJSON, _ := json.Marshal(testData)

		// For now, just show the test data and rule info
		// TODO: Implement proper condition evaluation for testing
		result := true // Placeholder

		var actions []models.Action
		json.Unmarshal(rule.Actions, &actions)

		c.HTML(http.StatusOK, "rule_test.html", gin.H{
			"Rule":       rule,
			"TestData":   testData,
			"TestResult": result,
			"Actions":    actions,
			"TestJSON":   string(testJSON),
		})
	})
}
