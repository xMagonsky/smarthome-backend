package internet_bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	PublicWS   string // ws://host:port/agent
	LocalURL   string // http://localhost:8081
	ServerID   string // unikalny ID agenta
	RetryDelay time.Duration
}

type requestMsg struct {
	Type   string      `json:"type"`
	ReqId  string      `json:"reqId"`
	Method string      `json:"method"`
	Path   string      `json:"path"`
	Body   interface{} `json:"body"`
}

type responseMsg struct {
	Type   string      `json:"type"`
	ReqId  string      `json:"reqId"`
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
}

func Start(cfg Config) {

	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 2 * time.Second
	}

	for {
		run(cfg)
		fmt.Println("Agent disconnected, reconnecting...")
		time.Sleep(cfg.RetryDelay)
	}
}

func run(cfg Config) {

	ws, _, err := websocket.DefaultDialer.Dial(cfg.PublicWS, nil)
	if err != nil {
		fmt.Println("WebSocket error:", err)
		return
	}
	defer ws.Close()

	// rejestracja agenta
	ws.WriteJSON(map[string]interface{}{
		"type": "register",
		"id":   cfg.ServerID,
	})

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}

		var req requestMsg
		json.Unmarshal(msg, &req)

		if req.Type != "request" {
			continue
		}

		respBody, status := doLocalRequest(cfg.LocalURL, req)

		ws.WriteJSON(responseMsg{
			Type:   "response",
			ReqId:  req.ReqId,
			Status: status,
			Body:   respBody,
		})
	}
}

// wykonanie requestu na lokalnym serwerze
func doLocalRequest(base string, req requestMsg) (interface{}, int) {
	bodyBytes, _ := json.Marshal(req.Body)

	httpReq, _ := http.NewRequest(req.Method, base+req.Path, bytes.NewBuffer(bodyBytes))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "local request failed", 500
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var parsed interface{}
	json.Unmarshal(raw, &parsed)

	return parsed, resp.StatusCode
}
