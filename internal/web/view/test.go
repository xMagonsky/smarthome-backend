package view

import (
	"net/http"
	"strings"
	"text/template"

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
}
