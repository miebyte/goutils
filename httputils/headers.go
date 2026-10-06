package httputils

import (
	"encoding/base64"
	"net/http"
)

type Header struct {
	http.Header
}

func NewHeader() Header {
	header := http.Header{}
	return Header{Header: header}
}

func (h *Header) Add(key, value string) *Header {
	h.Set(key, value)
	return h
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (h *Header) BasicAuth(user, pwd string) *Header {
	return h.Add("Authorization", "Basic "+basicAuth(user, pwd))
}

func DefaultJsonHeader() *Header {
	header := http.Header{}
	header.Add("Content-Type", "application/json")
	return &Header{Header: header}
}

func DefaultFormUrlEncodedHeader() *Header {
	header := http.Header{}
	header.Add("Content-Type", "application/x-www-form-urlencoded")
	return &Header{Header: header}
}

func DefaultFormHeader() *Header {
	header := http.Header{}
	header.Add("Content-Type", "multipart/form-data")
	return &Header{Header: header}
}
