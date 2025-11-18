package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/websocketutils"
)

type frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func main() {
	addr := flag.String("addr", "ws://localhost:8080/my-namespace", "websocket server url")
	flag.Parse()

	conn, _, err := websocket.DefaultDialer.Dial(*addr, nil)
	if err != nil {
		logging.Fatalf("dial error: %v", err)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go readLoop(ctx, conn)

	// 加入房间
	sendFrame(conn, websocketutils.EventJoinRoom, map[string]string{"room": "orders"})
	time.Sleep(time.Second)

	// 发送 hi 事件
	sendFrame(conn, "hi", map[string]string{"message": "hello server"})
	time.Sleep(time.Second)

	// 发送 broadcast 事件
	sendFrame(conn, "broadcast", map[string]string{
		"room":    "orders",
		"message": "hello all orders clients",
	})
	time.Sleep(time.Second)

	// 离开房间
	sendFrame(conn, websocketutils.EventLeaveRoom, map[string]string{"room": "orders"})
	time.Sleep(time.Second)

	// 发送 echo 事件
	time.Sleep(time.Second)
	sendFrame(conn, "echo", map[string]string{"message": "ping"})

	logging.Infof("press Ctrl+C to exit")
	<-ctx.Done()
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	time.Sleep(200 * time.Millisecond)
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
