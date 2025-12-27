package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Agent struct {
	ID  string
	WS  *websocket.Conn
	Mux sync.Mutex
}

var agents = map[string]*Agent{}
var agentsMux sync.Mutex

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type RequestMsg struct {
	Type    string            `json:"type"`
	ReqId   string            `json:"reqId"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

type ResponseMsg struct {
	Type   string      `json:"type"`
	ReqId  string      `json:"reqId"`
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
}

var pending = struct {
	m   map[string]chan ResponseMsg
	mux sync.Mutex
}{m: map[string]chan ResponseMsg{}}

func main() {
	r := gin.Default()

	r.GET("/agent", handleAgentWSGin)

	r.NoRoute(handleClientRequest)

	fmt.Println("Public server running on :5069")
	r.Run(":5069")
}

func handleAgentWSGin(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	var agentID string

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if agentID != "" {
				agentsMux.Lock()
				delete(agents, agentID)
				agentsMux.Unlock()
			}
			return
		}

		var data map[string]interface{}
		json.Unmarshal(msg, &data)

		switch data["type"] {
		case "register":
			agentID = data["id"].(string)
			fmt.Println("Agent registered:", agentID)

			agentsMux.Lock()
			agents[agentID] = &Agent{ID: agentID, WS: ws}
			agentsMux.Unlock()

		case "response":
			reqId := data["reqId"].(string)

			pending.mux.Lock()
			ch, ok := pending.m[reqId]
			if ok {
				ch <- ResponseMsg{
					Type:   "response",
					ReqId:  reqId,
					Status: int(data["status"].(float64)),
					Body:   data["body"],
				}
				delete(pending.m, reqId)
			}
			pending.mux.Unlock()
		}
	}
}

func handleClientRequest(c *gin.Context) {
	agentID := c.GetHeader("X-Server-ID")
	if agentID == "" {
		c.JSON(490, gin.H{"error": "Missing X-Server-ID"})
		return
	}

	agentsMux.Lock()
	agent, ok := agents[agentID]
	agentsMux.Unlock()

	if !ok {
		c.JSON(491, gin.H{"error": "Agent offline"})
		return
	}

	var body interface{}
	c.ShouldBindJSON(&body) // Ignore error - it's okay if there's no body

	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0] // Take the first value if multiple
		}
	}

	reqId := fmt.Sprintf("%d", time.Now().UnixNano())

	msg := RequestMsg{
		Type:    "request",
		ReqId:   reqId,
		Method:  c.Request.Method,
		Path:    c.Request.URL.Path,
		Headers: headers,
		Body:    body,
	}

	data, _ := json.Marshal(msg)

	agent.Mux.Lock()
	agent.WS.WriteMessage(websocket.TextMessage, data)
	agent.Mux.Unlock()

	respChan := make(chan ResponseMsg)
	pending.mux.Lock()
	pending.m[reqId] = respChan
	pending.mux.Unlock()

	select {
	case resp := <-respChan:
		c.JSON(resp.Status, resp.Body)

	case <-time.After(10 * time.Second):
		c.JSON(504, gin.H{"error": "Timeout"})
	}
}
