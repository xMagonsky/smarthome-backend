package main

import "smarthome/internal/web"

func main() {
	ws := web.NewWebServer()
	ws.Start(":5069")
}
