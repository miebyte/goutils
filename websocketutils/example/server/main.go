package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/websocketutils"
)

func main() {
	engine := ginutils.Default()
	ws := newServer()

	engine.GET("/ws", func(c *gin.Context) {
		ws.ServeHTTP(c.Writer, c.Request)
	})

	if err := engine.Run(":8080"); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to start server: %v", err)
	}
}

var (
	once     sync.Once
	wsServer *websocketutils.Server
)

type chatMessage struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

func newServer() *websocketutils.Server {
	once.Do(func() {
		wsServer = websocketutils.NewServer(
			websocketutils.WithPingInterval(2 * time.Second),
		)
		wsServer.On("chat", func(client *websocketutils.Client, event *websocketutils.Event) {
			var payload chatMessage
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				_ = client.EmitToNamespace(event.Namespace, "error", map[string]any{"error": "invalid payload"})
				return
			}
			fmt.Println("chat", logging.Jsonify(payload))

			if err := wsServer.EmitExcept("chat", map[string]any{
				"user":    payload.User,
				"message": payload.Message,
			}, client); err != nil {
				log.Printf("broadcast error: %v", err)
			}

			if event.Ack != nil {
				_ = event.Ack(map[string]any{"status": "ok"})
			}
		})
	})
	return wsServer
}
