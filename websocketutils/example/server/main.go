package main

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/websocketutils"
)

func main() {
	flags.Parse()

	srv := websocketutils.NewServer(
		websocketutils.WithHeartbeat(10*time.Second, 20*time.Second),
	)
	// 配置默认命名空间
	defaultNS := srv.Of("/")
	defaultNS.Use(func(socket websocketutils.Conn) error {
		logging.Infof("middleware: %s entering namespace %s", socket.ID(), socket.Namespace())
		return nil
	})

	defaultNS.On(websocketutils.EventConnection, func(s websocketutils.Conn) {
		logging.Infof("connection established: %s", s.ID())
	})

	orderNamespace := srv.Of("/orders")
	orderNamespace.On(websocketutils.EventConnection, func(socket websocketutils.Conn) {
		logging.Infof("connection established to orders namespace: %s", socket.ID())

		socket.On("join", func(s websocketutils.Conn) {
			rm := websocketutils.GetFrameData(s)
			type payload struct {
				Room string `json:"room"`
			}
			var p payload
			err := json.Unmarshal(rm, &p)
			if err != nil {
				logging.Errorf("unmarshal join room error: %v", err)
				return
			}

			if err := socket.Join(p.Room); err != nil {
				logging.Errorf("join room error: %v", err)
				return
			}
			logging.Infof("joined room: %s", p.Room)
		})

		socket.On("broadcast", func(s websocketutils.Conn) {
			orderNamespace.Emit("broadcast", map[string]string{
				"message": "hello all orders clients",
			})

			logging.Infof("broadcasted message: hello all orders clients")
		})

		socket.On("chat", func(s websocketutils.Conn) {
		})
	})

	engine := ginutils.Default()

	engine.GET("/:path", func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	})

	engine.Run(":8080")
}
