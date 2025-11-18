package main

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/websocketutils"
)

func main() {
	srv := websocketutils.NewServer()
	srv.Use(func(socket websocketutils.Socket) error {
		logging.Infof("middleware: %s entering namespace %s", socket.ID(), socket.Namespace())
		return nil
	})

	namespace := srv.Of("/my-namespace")
	namespace.Use(func(socket websocketutils.Socket) error {
		logging.Infof("ns middleware: established %s", socket.ID())
		return nil
	})

	namespace.On(websocketutils.EventConnection, func(socket websocketutils.Socket) {
		logging.Infof("connection established: %s", socket.ID())

		socket.On(websocketutils.EventJoinRoom, func(s websocketutils.Socket, data json.RawMessage) {
			var payload map[string]string
			_ = json.Unmarshal(data, &payload)
			roomName := payload["room"]
			if roomName == "" {
				return
			}
			if room := namespace.Room(roomName); room != nil {
				if err := room.EmitExcept("room:joined", map[string]string{
					"message": fmt.Sprintf("%s joined %s room", s.ID(), roomName),
				}, s); err != nil {
					logging.Errorf("room joined notify err: %v", err)
				}
			}

			logging.Infof("joined room: %s", roomName)
		})

		socket.On(websocketutils.EventLeaveRoom, func(s websocketutils.Socket, data json.RawMessage) {
			var payload map[string]string
			_ = json.Unmarshal(data, &payload)
			roomName := payload["room"]
			if roomName == "" {
				return
			}
			if room := namespace.Room(roomName); room != nil {
				if err := room.Emit("room:left", map[string]string{
					"message": fmt.Sprintf("%s left %s room", s.ID(), roomName),
				}); err != nil {
					logging.Errorf("room left notify err: %v", err)
				}
			}

			logging.Infof("left room: %s", roomName)
		})

		socket.On("hi", func(s websocketutils.Socket, data json.RawMessage) {
			var payload map[string]string
			_ = json.Unmarshal(data, &payload)
			logging.Infof("received hi from %s: %#v", s.ID(), payload)

			_ = s.Emit("hi:ack", map[string]string{
				"message": fmt.Sprintf("hello %s", s.ID()),
			})
		})

		socket.On("broadcast", func(s websocketutils.Socket, data json.RawMessage) {
			var payload map[string]string
			_ = json.Unmarshal(data, &payload)
			msg := payload["message"]
			if room := namespace.Room(payload["room"]); room != nil {
				if err := room.Emit("room:message", map[string]string{
					"from":    s.ID(),
					"message": msg + "broadcasted",
				}); err != nil {
					logging.Errorf("room emit err: %v", err)
				}
			}
		})

		socket.On("echo", func(s websocketutils.Socket, data json.RawMessage) {
			_ = s.Emit("echo", data)
		})
	})

	engine := ginutils.Default()

	engine.GET("/:namespace", func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	})

	engine.Run(":8080")
}
