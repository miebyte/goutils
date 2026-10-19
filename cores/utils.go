package cores

import (
	"context"

	"github.com/miebyte/goutils/internal/innerlog"
)

func prettyErrorCheck(ctx context.Context, err error, message string) bool {
	if err != nil {
		innerlog.Logger.Errorc(ctx, "%s. error: %v", message, err)
		return false
	}

	return true
}
