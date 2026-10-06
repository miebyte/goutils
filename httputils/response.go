package httputils

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	"github.com/pkg/errors"
)

type Response struct {
	*http.Response
	err error
}

func NewResponse(resp *http.Response, err error) *Response {
	return &Response{
		Response: resp,
		err:      err,
	}
}

func (r *Response) Error() error {
	return r.err
}

func (r *Response) BodyClose() error {
	return r.Body.Close()
}

func (r *Response) BodyString() (string, error) {
	bytes, err := r.BodyBytes()
	if err != nil {
		return "", errors.Wrap(err, "[BodyString] body bytes")
	}
	return string(bytes), nil
}

func (r *Response) BodyBytes() ([]byte, error) {
	if r.err != nil {
		return []byte{}, r.err
	}

	if r.Response == nil || r.Response.Body == nil {
		return []byte{}, errors.New("[BodyBytes] response or response body is nil")
	}
	defer r.Response.Body.Close()

	bytes, err := io.ReadAll(r.Response.Body)
	if err != nil {
		return []byte{}, errors.Wrap(err, "[BodyBytes] read response body")
	}

	return bytes, nil
}

func (r *Response) BodyJson(v any) error {
	if r.err != nil {
		return r.err
	}

	if v == nil {
		return errors.New("[BodyJson] value is nil")
	}

	if reflect.TypeOf(v).Kind() != reflect.Pointer {
		return errors.New("[BodyJson] value is not ptr")
	}

	bytes, err := r.BodyBytes()
	if err != nil {
		return errors.Wrap(err, "[BodyJson] body bytes")
	}
	return json.Unmarshal(bytes, v)
}

// DecodeJSON decodes the response body into T and returns the typed result.
func (r *Response) DecodeJSON[T any]() (T, error) {
	var result T
	if err := r.BodyJson(&result); err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

func (r *Response) IsSuccess() bool {
	if r.err != nil || r.Response == nil {
		return false
	}
	return r.Response.StatusCode >= 200 && r.Response.StatusCode < 300
}

func (r *Response) StatusCode() int {
	if r.Response == nil {
		return 0
	}
	return r.Response.StatusCode
}
