package ginutils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestBodyBindUnknownContentLength(t *testing.T) {
	c, _ := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader(`{"name":"alice"}`)))
	c.Request.ContentLength = -1
	c.Request.TransferEncoding = []string{"chunked"}
	c.Request.Header.Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
	}
	if err := bindRequestData(c, &req, []bindStrategy{&bodyBind{}}); err != nil {
		t.Fatal(err)
	}
	if req.Name != "alice" {
		t.Fatalf("name = %q, want alice", req.Name)
	}
}

func TestBodyBindNeed(t *testing.T) {
	for _, tc := range []struct {
		name string
		body io.ReadCloser
		want bool
	}{
		{name: "nil body", body: nil, want: false},
		{name: "no body", body: http.NoBody, want: false},
		{name: "stream without content length", body: io.NopCloser(strings.NewReader(`{}`)), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestContext()
			c.Request.Body = tc.body
			if got := (&bodyBind{}).Need(c); got != tc.want {
				t.Fatalf("Need() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestQueryBindEmptyQueryAppliesDefault(t *testing.T) {
	c, _ := newTestContext()
	var req struct {
		Limit int `form:"limit,default=20"`
	}
	if err := bindRequestData(c, &req, []bindStrategy{&queryBind{}}); err != nil {
		t.Fatal(err)
	}
	if req.Limit != 20 {
		t.Fatalf("limit = %d, want 20", req.Limit)
	}
}

func TestHeaderBindEmptyHeaderAppliesDefault(t *testing.T) {
	c, _ := newTestContext()
	var req struct {
		Locale string `header:"X-Locale,default=zh"`
	}
	if err := bindRequestData(c, &req, []bindStrategy{&headerBind{}}); err != nil {
		t.Fatal(err)
	}
	if req.Locale != "zh" {
		t.Fatalf("locale = %q, want zh", req.Locale)
	}
}

func TestQueryBindEmptyQueryKeepsFormValue(t *testing.T) {
	c, _ := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("limit=50"))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var req struct {
		Limit int `form:"limit,default=20"`
	}
	if err := bindRequestData(c, &req, resolveStrategies(reflect.TypeOf(req))); err != nil {
		t.Fatal(err)
	}
	if req.Limit != 50 {
		t.Fatalf("limit = %d, want 50", req.Limit)
	}
}
