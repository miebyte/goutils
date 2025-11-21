package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

type Frame struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/orders"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	// Join room
	joinPayload := map[string]string{"room": "room1"}
	joinBytes, _ := json.Marshal(joinPayload)
	joinFrame := Frame{Event: "join", Data: joinBytes}
	joinMsg, _ := json.Marshal(joinFrame)
	err = c.WriteMessage(websocket.TextMessage, joinMsg)
	if err != nil {
		log.Println("write join:", err)
		return
	}

	// Broadcast
	time.Sleep(time.Second)
	broadcastFrame := Frame{Event: "broadcast", Data: nil}
	broadcastMsg, _ := json.Marshal(broadcastFrame)
	err = c.WriteMessage(websocket.TextMessage, broadcastMsg)
	if err != nil {
		log.Println("write broadcast:", err)
		return
	}

	select {
	case <-interrupt:
		log.Println("interrupt")
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("write close:", err)
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	case <-time.After(5 * time.Second):
		log.Println("test finished")
	}
}
