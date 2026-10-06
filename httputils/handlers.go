package httputils

import (
	"context"
	"io"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/miebyte/goutils/logging"
	"github.com/pkg/errors"
)

type HandleFunc func(c *RequestContext)
type HandlersChain []HandleFunc

func HandlerDuration() HandleFunc {
	return func(c *RequestContext) {
		startTime := time.Now()
		c.Next()
		c.duration = time.Since(startTime)
		logging.Infoc(
			c.ctx,
			"[http] %v | url: %v, params: %v, duration: %v, err: %v",
			c.method, c.url, logging.JsonifyNoIndent(c.queryParams), c.duration, c.err,
		)
	}
}

func defaultHTTPHandler() HandleFunc {
	return func(c *RequestContext) {
		resp, err := c.client.Do(c.request)
		c.response = resp
		if err != nil {
			c.err = multierror.Append(c.err, errors.Wrap(err, "do request"))
			c.Abort()
			return
		}

		// ensure cancel is called when response body is closed
		if c.cancel != nil && c.response != nil && c.response.Body != nil {
			c.response.Body = &cancelOnCloseReadCloser{
				ReadCloser: c.response.Body,
				cancel:     c.cancel,
			}
		}
	}
}

// cancelOnCloseReadCloser 在关闭响应体时触发上下文取消
type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if c.cancel != nil {
		c.cancel()
	}
	return err
}
