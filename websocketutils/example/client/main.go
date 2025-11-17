package main

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type wireMessage struct {
	Type      string         `json:"type"`
	Namespace string         `json:"namespace,omitempty"`
	Event     string         `json:"event,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	AckID     string         `json:"ackId,omitempty"`
	Room      string         `json:"room,omitempty"`
}

var ackSeq atomic.Uint64

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", http.Header{})
	if err != nil {
		log.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	writeMu := &sync.Mutex{}

	go readLoop(conn)
	go chatRoomLoop(conn, writeMu, "/", "lobby", "example-client")
	// go notifyLoop(conn, writeMu, "/")

	pingLoop(conn, writeMu)
}

func chatRoomLoop(conn *websocket.Conn, mu *sync.Mutex, namespace, room, user string) {
	if err := sendJSON(conn, mu, wireMessage{Type: "join", Namespace: namespace, Room: room}); err != nil {
		log.Printf("join room failed: %v", err)
		return
	}
	log.Printf("joined room %s", room)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	counter := 0
	for range ticker.C {
		counter++
		ackID := newAckID()
		payload := wireMessage{
			Type:      "event",
			Namespace: namespace,
			Event:     "chat",
			AckID:     ackID,
			Room:      room,
			Data: map[string]any{
				"user":    user,
				"message": "room message #" + strconv.Itoa(counter),
			},
		}
		if err := sendJSON(conn, mu, payload); err != nil {
			log.Printf("send room message failed: %v", err)
			return
		}
		log.Printf("sent chat message %d with ack %s", counter, ackID)
		break
	}
}

func notifyLoop(conn *websocket.Conn, mu *sync.Mutex, namespace string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	counter := 0
	for range ticker.C {
		counter++
		msg := wireMessage{
			Type:      "event",
			Namespace: namespace,
			Event:     "notify",
			Data: map[string]any{
				"sequence": counter,
				"content":  "broadcast event",
			},
		}
		if err := sendJSON(conn, mu, msg); err != nil {
			log.Printf("send notify failed: %v", err)
			return
		}
	}
}

func pingLoop(conn *websocket.Conn, mu *sync.Mutex) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := sendJSON(conn, mu, wireMessage{Type: "ping"}); err != nil {
			log.Printf("ping failed: %v", err)
			return
		}
	}
}

func readLoop(conn *websocket.Conn) {
	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("read error: %v", err)
			return
		}
		log.Printf("recv %#v", msg)
	}
}

func sendJSON(conn *websocket.Conn, mu *sync.Mutex, v any) error {
	mu.Lock()
	defer mu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return conn.WriteJSON(v)
}

func newAckID() string {
	id := ackSeq.Add(1)
	return "client-ack-" + strconv.FormatUint(id, 10)
}
