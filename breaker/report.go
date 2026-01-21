package breaker

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miebyte/goutils/debounce"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/internal/share"
)

var (
	lessExecutor = debounce.NewLessExecutor(time.Minute * 5)
	dropped      int32
)

// Report reports given message.
func Report(msg string) {
	clusterName := share.GetServiceName()

	reported := lessExecutor.DoOrDiscard(func() {
		var builder strings.Builder
		builder.WriteString(fmt.Sprintln(time.Now().Format(time.DateTime)))
		if len(clusterName) > 0 {
			builder.WriteString(fmt.Sprintf("cluster: %s\n", clusterName))
		}
		builder.WriteString(fmt.Sprintf("host: %s\n", share.GetHostName()))
		dp := atomic.SwapInt32(&dropped, 0)
		if dp > 0 {
			builder.WriteString(fmt.Sprintf("dropped: %d\n", dp))
		}
		builder.WriteString(strings.TrimSpace(msg))
		innerlog.Logger.Error(builder.String())
	})
	if !reported {
		atomic.AddInt32(&dropped, 1)
	}
}
