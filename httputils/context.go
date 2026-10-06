package httputils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

const abortIndex = math.MaxInt8 >> 1

type RequestContext struct {
	ctx    context.Context
	cancel context.CancelFunc

	url     string
	method  string
	index   int8
	timeout time.Duration

	handlers HandlersChain

	headers     Header
	queryParams Params
	body        io.Reader

	client   *http.Client
	request  *http.Request
	response *http.Response
	duration time.Duration
	err      error
}

func (c *RequestContext) AddError(err error) {
	c.err = multierror.Append(c.err, err)
}

func (c *RequestContext) GetError() error {
	return c.err
}

func (c *RequestContext) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *RequestContext) Abort() {
	c.index = abortIndex
}

func (rc *RequestContext) SetRequest(req *http.Request) *RequestContext {
	rc.request = req
	return rc
}

// SetURL 设置请求URL
func (rc *RequestContext) SetURL(url string) *RequestContext {
	rc.url = url
	return rc
}

// SetHeader 设置单个请求头
func (rc *RequestContext) SetHeader(key, value string) *RequestContext {
	rc.headers.Add(key, value)
	return rc
}

// SetHeaders 设置多个请求头
func (rc *RequestContext) SetHeaders(headers map[string]string) *RequestContext {
	for k, v := range headers {
		rc.headers.Add(k, v)
	}
	return rc
}

// SetQueryParam 设置单个查询参数
func (rc *RequestContext) SetQueryParam(key, value string) *RequestContext {
	rc.queryParams[key] = value
	return rc
}

// SetQueryParams 设置多个查询参数
func (rc *RequestContext) SetQueryParams(params Params) *RequestContext {
	maps.Copy(rc.queryParams, params)
	return rc
}

// SetBody 设置请求体
func (rc *RequestContext) SetBody(body io.Reader) *RequestContext {
	rc.body = body
	return rc
}

// SetJSONBody 设置JSON请求体
func (rc *RequestContext) SetJSONBody(data any) *RequestContext {
	jsonData, err := json.Marshal(data)
	if err != nil {
		rc.AddError(fmt.Errorf("failed to marshal JSON body: %w", err))
		return rc
	}
	rc.body = bytes.NewReader(jsonData)
	rc.SetHeader("Content-Type", "application/json")
	return rc
}

func (rc *RequestContext) SetTimeout(timeout time.Duration) *RequestContext {
	rc.timeout = timeout
	return rc
}

func (rc *RequestContext) buildRequest(method string) (*http.Request, error) {
	if rc.request != nil {
		return rc.request, nil
	}

	if rc.url == "" {
		return nil, fmt.Errorf("URL is required")
	}

	parsedURL, err := url.Parse(rc.url)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if len(rc.queryParams) > 0 {
		q := parsedURL.Query()
		for k, v := range rc.queryParams {
			q.Set(k, v)
		}
		parsedURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(rc.ctx, method, parsedURL.String(), rc.body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header = rc.headers.Header

	return req, nil
}

func (rc *RequestContext) executeRequest(ctx context.Context, method string) *Response {
	// 先处理链式调用过程中的错误
	if rc.err != nil {
		return NewResponse(nil, errors.Wrap(rc.err, "build chain error"))
	}

	rc.ctx = ctx
	if rc.timeout > 0 {
		rc.ctx, rc.cancel = context.WithTimeout(ctx, rc.timeout)
	}

	if rc.request == nil {
		req, err := rc.buildRequest(method)
		if err != nil {
			// ensure cancel is called when error occurs
			if rc.cancel != nil {
				rc.cancel()
			}
			err = errors.Wrap(err, "build request")
			return NewResponse(nil, err)
		}
		rc.SetRequest(req)
	}

	rc.handlers = append(rc.handlers, defaultHTTPHandler())
	rc.Next()

	if rc.cancel != nil {
		if rc.err != nil || (rc.response == nil || rc.response.Body == nil) {
			rc.cancel()
		}
	}

	return NewResponse(rc.response, rc.err)
}

// Get 发送GET请求
func (rc *RequestContext) Get(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodGet)
}

// Post 发送POST请求
func (rc *RequestContext) Post(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodPost)
}

// Put 发送PUT请求
func (rc *RequestContext) Put(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodPut)
}

// Delete 发送DELETE请求
func (rc *RequestContext) Delete(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodDelete)
}

// Patch 发送PATCH请求
func (rc *RequestContext) Patch(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodPatch)
}

// Head 发送HEAD请求
func (rc *RequestContext) Head(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodHead)
}

// Options 发送OPTIONS请求
func (rc *RequestContext) Options(ctx context.Context) *Response {
	return rc.executeRequest(ctx, http.MethodOptions)
}
