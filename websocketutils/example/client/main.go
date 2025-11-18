package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
)

var (
	ports = flags.Int("port", 8081, "port")
)

type frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func main() {
	flags.Parse()

	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/orders", nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go readLoop(context.Background(), conn)

	engine := ginutils.Default()
	engine.GET("/join", func(c *gin.Context) {
		room := c.Query("room")
		if room == "" {
			room = "orders"
		}
		sendFrame(conn, "join", map[string]string{"room": room})
	})

	engine.GET("/broadcast", func(c *gin.Context) {
		sendFrame(conn, "broadcast", map[string]string{
			"message": "hello all orders clients",
		})
	})

	logging.PanicError(engine.Run(fmt.Sprintf(":%d", ports())))
}

func readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var f frame
			if err := conn.ReadJSON(&f); err != nil {
				logging.Errorf("read error: %v", err)
				return
			}
			logging.Infof("event=%s payload=%s", f.Event, string(f.Data))
		}
	}
}

func sendFrame(conn *websocket.Conn, event string, payload any) {
	msg := frame{Event: event}
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			logging.Errorf("marshal error: %v", err)
			return
		}
		msg.Data = data
	}
	if err := conn.WriteJSON(msg); err != nil {
		logging.Errorf("write error: %v", err)
	} else {
		logging.Infof("sent event=%s", event)
	}
}
