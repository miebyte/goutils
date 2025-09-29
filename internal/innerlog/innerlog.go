package innerlog

import (
	"os"

	"github.com/miebyte/goutils/logging"
)

var (
	Logger *logging.PrettyLogger
)

func init() {
	Logger = logging.NewPrettyLogger(os.Stdout, logging.WithModule("GOUTILS"))
	Logger.SetWithSource(false)
}
