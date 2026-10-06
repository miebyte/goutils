package httputils

import (
	"context"
	"io"
	"net/http"
	"slices"
	"time"
)

type clientOption struct {
	timeout time.Duration
	tp      http.RoundTripper
}

type clientOptionFn func(*clientOption)

func WithTimeout(timeout time.Duration) clientOptionFn {
	return func(opt *clientOption) {
		opt.timeout = timeout
	}
}

func WithTransport(tp http.RoundTripper) clientOptionFn {
	return func(opt *clientOption) {
		opt.tp = tp
	}
}

type Client struct {
	*http.Client
	handlers HandlersChain
}

func NewClient(opts ...clientOptionFn) *Client {
	opt := &clientOption{
		tp: http.DefaultTransport,
	}
	for _, o := range opts {
		o(opt)
	}

	client := &http.Client{
		Transport: opt.tp,
		Timeout:   opt.timeout,
	}

	return &Client{
		Client:   client,
		handlers: make(HandlersChain, 0),
	}
}

func (c *Client) Use(handlers ...HandleFunc) *Client {
	c.handlers = append(c.handlers, handlers...)
	return c
}

func (c *Client) Start() *RequestContext {
	return &RequestContext{
		client:      c.Client,
		headers:     NewHeader(),
		queryParams: NewParams(),
		index:       -1,
		handlers:    slices.Clone(c.handlers),
	}
}

func Get(ctx context.Context, url string) (*http.Response, error) {
	c := NewClient()

	resp := c.Start().
		SetURL(url).
		Get(ctx)
	if resp.Error() != nil {
		return nil, resp.Error()
	}

	return resp.Response, nil
}

func Post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	c := NewClient()

	resp := c.Start().
		SetURL(url).
		SetBody(body).
		Post(ctx)
	if resp.Error() != nil {
		return nil, resp.Error()
	}

	return resp.Response, nil
}

func Put(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	c := NewClient()

	resp := c.Start().
		SetURL(url).
		SetBody(body).
		Put(ctx)
	if resp.Error() != nil {
		return nil, resp.Error()
	}

	return resp.Response, nil
}

func Delete(ctx context.Context, url string) (*http.Response, error) {
	c := NewClient()

	resp := c.Start().
		SetURL(url).
		Delete(ctx)
	if resp.Error() != nil {
		return nil, resp.Error()
	}

	return resp.Response, nil
}
