package websocketutils

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServerNamespaceAndRoom(t *testing.T) {
	server := NewServer()
	ns := server.Of("chat")

	connCh := make(chan Conn, 1)
	errCh := make(chan error, 1)
	pingCh := make(chan map[string]string, 1)

	ns.On(EventConnection, func(ctx *Context) {
		connCh <- ctx.Conn()
		if err := ctx.Conn().Emit("welcome", map[string]string{"msg": "hi"}); err != nil {
			errCh <- err
		}
	})

	ns.On("ping", func(ctx *Context) {
		payload := map[string]string{}
		if err := ctx.Scan(&payload); err != nil {
			errCh <- err
			return
		}
		pingCh <- payload
		if err := ctx.Conn().Emit("pong", payload); err != nil {
			errCh <- err
		}
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	frame := readFrame(t, ws)
	if frame.Event != "welcome" {
		t.Fatalf("unexpected event %s", frame.Event)
	}
	var welcome map[string]string
	if err := json.Unmarshal(frame.Data, &welcome); err != nil {
		t.Fatalf("unmarshal welcome: %v", err)
	}
	if welcome["msg"] != "hi" {
		t.Fatalf("unexpected welcome payload: %+v", welcome)
	}

	var serverConn Conn
	select {
	case serverConn = <-connCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for server conn")
	}

	if err := serverConn.Join("room-x"); err != nil {
		t.Fatalf("join room failed: %v", err)
	}

	ns.Room("room-x").Broadcast("notice", map[string]string{"msg": "room"})
	frame = readFrame(t, ws)
	if frame.Event != "notice" {
		t.Fatalf("unexpected room event %s", frame.Event)
	}

	writeFrame(t, ws, "ping", map[string]string{"msg": "ping"})

	select {
	case <-pingCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ping handler")
	}

	frame = readFrame(t, ws)
	if frame.Event != "pong" {
		t.Fatalf("unexpected pong event %s", frame.Event)
	}

	select {
	case err := <-errCh:
		t.Fatalf("handler error: %v", err)
	default:
	}
}

func TestServerNamespacePrefix(t *testing.T) {
	server := NewServer(WithNamespacePrefix("/socket"))
	chatNS := server.Of("chat")
	defaultNS := server.Of(defaultNamespaceName)

	chatNS.On(EventConnection, func(ctx *Context) {
		_ = ctx.Conn().Emit("prefixed", map[string]string{"ns": ctx.Namespace().Name()})
	})

	defaultNS.On(EventConnection, func(ctx *Context) {
		_ = ctx.Conn().Emit("fallback", map[string]string{"ns": ctx.Namespace().Name()})
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	prefixURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/socket/chat"
	ws1, _, err := websocket.DefaultDialer.Dial(prefixURL, nil)
	if err != nil {
		t.Fatalf("dial with prefix failed: %v", err)
	}
	defer ws1.Close()

	frame := readFrame(t, ws1)
	if frame.Event != "prefixed" {
		t.Fatalf("unexpected prefixed event %s", frame.Event)
	}

	noPrefixURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat"
	ws2, _, err := websocket.DefaultDialer.Dial(noPrefixURL, nil)
	if err != nil {
		t.Fatalf("dial without prefix failed: %v", err)
	}
	defer ws2.Close()

	frame = readFrame(t, ws2)
	if frame.Event != "fallback" {
		t.Fatalf("unexpected fallback event %s", frame.Event)
	}
}

type ctxKey string

func TestServerAllowRequestFuncInjectsContext(t *testing.T) {
	key := ctxKey("user")
	server := NewServer(WithAllowRequestFunc(func(r *http.Request) (*http.Request, error) {
		if r.URL.Query().Get("token") != "ok" {
			return nil, errors.New("blocked")
		}
		ctx := context.WithValue(r.Context(), key, "uid-1")
		return r.WithContext(ctx), nil
	}))

	ns := server.Of("chat")
	valCh := make(chan string, 1)
	ns.On(EventConnection, func(ctx *Context) {
		req := ctx.Conn().Request()
		if req == nil {
			valCh <- ""
			return
		}
		if v, _ := req.Context().Value(key).(string); v != "" {
			valCh <- v
		} else {
			valCh <- ""
		}
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat?token=ok"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	select {
	case val := <-valCh:
		if val != "uid-1" {
			t.Fatalf("unexpected context value %s", val)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for context value")
	}
}

func TestNamespaceRoomContextEmit(t *testing.T) {
	server := NewServer()
	ns := server.Of("chat")

	readyCh := make(chan struct{}, 3)
	connMu := sync.Mutex{}
	conns := make(map[string]Conn)

	ns.On(EventConnection, func(ctx *Context) {
		user := ctx.Conn().Request().URL.Query().Get("user")
		switch user {
		case "a":
			_ = ctx.Conn().Join("room-1")
		case "b":
			_ = ctx.Conn().Join("room-1")
			_ = ctx.Conn().Join("room-2")
		case "c":
			_ = ctx.Conn().Join("room-2")
		}
		connMu.Lock()
		conns[user] = ctx.Conn()
		connMu.Unlock()
		readyCh <- struct{}{}
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	baseURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat"
	wsA := dialWS(t, baseURL+"?user=a")
	defer wsA.Close()
	wsB := dialWS(t, baseURL+"?user=b")
	defer wsB.Close()
	wsC := dialWS(t, baseURL+"?user=c")
	defer wsC.Close()

	for i := 0; i < 3; i++ {
		select {
		case <-readyCh:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for room joins")
		}
	}

	if err := ns.To("room-1").To("room-2").Emit("notice", map[string]string{"msg": "broadcast"}); err != nil {
		t.Fatalf("room context emit failed: %v", err)
	}

	for _, ws := range []*websocket.Conn{wsA, wsB, wsC} {
		frame := readFrame(t, ws)
		if frame.Event != "notice" {
			t.Fatalf("unexpected event %s", frame.Event)
		}
	}

	connMu.Lock()
	excluded := conns["b"]
	connMu.Unlock()
	if excluded == nil {
		t.Fatal("missing connection for exclusion")
	}

	if err := ns.To("room-1").To("room-2").EmitExcept("partial", map[string]string{"msg": "subset"}, excluded); err != nil {
		t.Fatalf("room context emit except failed: %v", err)
	}

	for _, ws := range []*websocket.Conn{wsA, wsC} {
		frame := readFrame(t, ws)
		if frame.Event != "partial" {
			t.Fatalf("unexpected subset event %s", frame.Event)
		}
	}

	if err := wsB.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if _, _, err := wsB.ReadMessage(); err == nil {
		t.Fatal("expected timeout for excluded connection")
	} else {
		if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
			t.Fatalf("expected timeout error, got %v", err)
		}
	}
}

func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return ws
}

func TestServerHeartbeatKeepsAlive(t *testing.T) {
	server := NewServer(WithHeartbeat(50*time.Millisecond, 200*time.Millisecond))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	for i := 0; i < 3; i++ {
		frame := readFrame(t, ws)
		if frame.Event != "ping" {
			t.Fatalf("unexpected event %s", frame.Event)
		}
		writeFrame(t, ws, "pong", nil)
	}
}

func TestServerHeartbeatTimeout(t *testing.T) {
	server := NewServer(WithHeartbeat(40*time.Millisecond, 120*time.Millisecond))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	}))
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/chat"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	frame := readFrame(t, ws)
	if frame.Event != "ping" {
		t.Fatalf("expected ping event, got %s", frame.Event)
	}

	deadline := time.Now().Add(800 * time.Millisecond)
	for {
		if err := ws.SetReadDeadline(deadline); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		if _, _, err := ws.ReadMessage(); err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("heartbeat timeout did not close connection")
		}
	}
}

func readFrame(t *testing.T, ws *websocket.Conn) Frame {
	t.Helper()
	if err := ws.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}
	var frame Frame
	if err := json.Unmarshal(data, &frame); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	return frame
}

func writeFrame(t *testing.T, ws *websocket.Conn, event string, payload any) {
	t.Helper()
	frame := Frame{Event: event}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		frame.Data = raw
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write frame: %v", err)
	}
}
