package websocketutils

import (
	"os"

	"github.com/miebyte/goutils/logging"
)

var (
	websocketLogger logging.Logger
)

func init() {
	websocketLogger = logging.NewPrettyLogger(
		os.Stdout,
		logging.WithModule("WebsocketUtils"),
		logging.WithEnableSource(false),
	)
}

func SetLogger(logger logging.Logger) {
	websocketLogger = logger
}

func Logger() logging.Logger {
	return websocketLogger
}
