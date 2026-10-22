package consul

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/miebyte/goutils/flags/v2/provider"
)

func TestNewConsulClientDoesNotModifySharedConfig(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "consul.example"}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	config := &consulapi.Config{Address: "127.0.0.1:8500", Transport: transport}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			source, err := NewConsulClient(config, "")
			if err != nil {
				t.Error(err)
				return
			}
			_ = source.Close()
		})
	}
	wg.Wait()
	if transport.TLSClientConfig != tlsConfig || config.HttpClient != nil || config.Scheme != "" {
		t.Fatal("constructing providers modified the shared client configuration")
	}
}

func TestReadConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		raw      string
		nilValue bool
	}{
		{"raw value", http.StatusOK, `{"limit":3}`, false},
		{"invalid JSON stays raw", http.StatusOK, `invalid`, false},
		{"empty value is present", http.StatusOK, "", true},
		{"missing key", http.StatusNotFound, "", false},
		{"permission error", http.StatusForbidden, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/kv/custom/app" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				writeConsulResponse(w, "custom/app", consulResponse{index: "1", status: tc.status, raw: tc.raw, nilValue: tc.nilValue})
			}))
			t.Cleanup(server.Close)
			source := newTestConsulProvider(t, server, "prefix")
			raw, err := source.ReadConfig(context.Background(), "custom/app")
			if tc.status == http.StatusOK {
				if err != nil || string(raw) != tc.raw {
					t.Fatalf("ReadConfig = %s, %v; want %s", raw, err, tc.raw)
				}
				return
			}
			if err == nil || errors.Is(err, provider.ErrConfigNotFound) != (tc.status == http.StatusNotFound) {
				t.Fatalf("ReadConfig status %d error = %v", tc.status, err)
			}
		})
	}
}

func TestReadConfigCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	source := newTestConsulProvider(t, server, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.ReadConfig(ctx, "app"); !errors.Is(err, context.Canceled) || errors.Is(err, provider.ErrConfigNotFound) {
		t.Fatalf("canceled ReadConfig error = %v", err)
	}
}

func TestWatchConfigChangesAndRecovers(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "10", raw: `{"limit":1}`})
	request := nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)

	request.respond <- consulResponse{index: "11", raw: `{"limit":1}`}
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "12", raw: `{"limit":2}`}
	assertConsulValue(t, events, `{"limit":2}`)

	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "13", status: http.StatusNotFound}
	if event := nextConsulEvent(t, events); event.Path != "config/app" || !errors.Is(event.Err, provider.ErrConfigNotFound) {
		t.Fatalf("deleted key event = %+v", event)
	}
	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "14", raw: `{"limit":3}`}
	assertConsulValue(t, events, `{"limit":3}`)

	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{disconnect: true}
	request = nextConsulRequestWithin(t, requests, 10*time.Second)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "15", raw: `{"limit":3}`}
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "16", raw: `{"limit":4}`}
	assertConsulValue(t, events, `{"limit":4}`)
}

func TestWatchConfigInitiallyMissing(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "1", status: http.StatusNotFound})
	request := nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "2", raw: `{"limit":1}`}
	assertConsulValue(t, events, `{"limit":1}`)
}

func TestWatchConfigWaitsForInitialSnapshot(t *testing.T) {
	source, requests := newScriptedConsul(t)
	result := beginConsulWatch(source)
	request := nextConsulRequest(t, requests)
	select {
	case result := <-result:
		t.Fatalf("WatchConfig returned before the initial response: %+v", result)
	case <-time.After(50 * time.Millisecond):
	}
	request.respond <- consulResponse{index: "1", raw: `1`}
	events := finishConsulWatch(t, result)
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "2", raw: `2`}
	assertConsulValue(t, events, `2`)
}

func TestWatchConfigEmptyValueIsPresent(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "1", status: http.StatusNotFound})
	request := nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "2", nilValue: true}
	assertConsulValue(t, events, "")
	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "3", nilValue: true}
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "4", status: http.StatusNotFound}
	if event := nextConsulEvent(t, events); event.Path != "config/app" || !errors.Is(event.Err, provider.ErrConfigNotFound) {
		t.Fatalf("empty value deleted event = %+v", event)
	}
}

func TestWatchConfigEventDoesNotExposeSDKSnapshot(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "1", raw: `0`})
	request := nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "2", raw: `1`}
	event := nextConsulEvent(t, events)
	if event.Err != nil || string(event.Config) != "1" {
		t.Fatalf("config event = %+v", event)
	}
	event.Config[0] = '9'
	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "3", raw: `1`}
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "4", raw: `2`}
	assertConsulValue(t, events, `2`)
}

func TestWatchConfigInitialFailureRecovers(t *testing.T) {
	source, requests := newScriptedConsul(t)
	result := beginConsulWatch(source)
	nextConsulRequest(t, requests).respond <- consulResponse{status: http.StatusForbidden}
	request := nextConsulRequestWithin(t, requests, 10*time.Second)
	request.respond <- consulResponse{index: "1", raw: `1`}
	events := finishConsulWatch(t, result)
	request = nextConsulRequest(t, requests)
	assertNoConsulEvent(t, events)
	request.respond <- consulResponse{index: "2", raw: `2`}
	assertConsulValue(t, events, `2`)
}

func TestWatchConfigRejectsEmptyKey(t *testing.T) {
	source, _ := newScriptedConsul(t)
	events, err := source.WatchConfig("")
	if events != nil || err == nil || errors.Is(err, provider.ErrConfigNotFound) {
		t.Fatalf("WatchConfig empty key = %v, %v", events, err)
	}
}

func TestCloseCancelsWatchWithUnreadEvents(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "1", raw: `0`})
	request := nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "2", raw: `1`}
	request = nextConsulRequest(t, requests)
	request.respond <- consulResponse{index: "3", raw: `2`}
	closed := make(chan error, 1)
	go func() {
		if err := source.Close(); err != nil {
			closed <- err
			return
		}
		closed <- source.Close()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close blocked with unread events")
	}
	for {
		select {
		case _, ok := <-events:
			if !ok {
				if stream, err := source.WatchConfig("app"); stream != nil || !errors.Is(err, os.ErrClosed) {
					t.Fatalf("watch after Close = %v, %v", stream, err)
				}
				return
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Close did not close the event stream")
		}
	}
}

func TestCloseCancelsBlockingQuery(t *testing.T) {
	source, requests := newScriptedConsul(t)
	events := startConsulWatch(t, source, requests, consulResponse{index: "1", raw: `0`})
	request := nextConsulRequest(t, requests)
	closed := make(chan error, 1)
	go func() { closed <- source.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close blocked with an active query")
	}
	select {
	case <-request.canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not cancel the blocking request")
	}
	if _, ok := <-events; ok {
		t.Fatal("event stream stayed open after Close")
	}
}

func TestCloseCancelsInitialWatchQuery(t *testing.T) {
	for i := range 20 {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			source, requests := newScriptedConsul(t)
			result := beginConsulWatch(source)
			request := nextConsulRequest(t, requests)
			closed := make(chan error, 1)
			go func() { closed <- source.Close() }()
			select {
			case err := <-closed:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Close blocked on the initial watch query")
			}
			select {
			case <-request.canceled:
			case <-time.After(3 * time.Second):
				t.Fatal("Close did not cancel the initial watch query")
			}
			select {
			case result := <-result:
				if result.events != nil || result.err == nil || errors.Is(result.err, provider.ErrConfigNotFound) {
					t.Fatalf("canceled WatchConfig = %v, %v", result.events, result.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("WatchConfig stayed blocked after Close")
			}
		})
	}
}

func TestWatchConfigAfterCloseDoesNotRequestConsul(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		writeConsulResponse(w, "config/app", consulResponse{index: "1", raw: `1`})
	}))
	t.Cleanup(server.Close)
	source := newTestConsulProvider(t, server, "")
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	events, err := source.WatchConfig("config/app")
	if events != nil || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("WatchConfig after Close = %v, %v", events, err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("WatchConfig after Close made %d HTTP requests", got)
	}
}

type consulResponse struct {
	index      string
	status     int
	raw        string
	nilValue   bool
	disconnect bool
}

type consulWatchResult struct {
	events <-chan provider.Event
	err    error
}

type consulRequest struct {
	respond  chan consulResponse
	canceled <-chan struct{}
}

func newTestConsulProvider(t *testing.T, server *httptest.Server, prefix string) *ConsulProvider {
	t.Helper()
	source, err := NewConsulClient(&consulapi.Config{
		Address:    strings.TrimPrefix(server.URL, "http://"),
		Scheme:     "http",
		HttpClient: server.Client(),
	}, prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source
}

func newScriptedConsul(t *testing.T) (*ConsulProvider, <-chan consulRequest) {
	t.Helper()
	requests := make(chan consulRequest)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/kv/config/app" {
			t.Errorf("watch request = %s %s", r.Method, r.URL.Path)
		}
		request := consulRequest{
			respond: make(chan consulResponse, 1), canceled: r.Context().Done(),
		}
		select {
		case requests <- request:
		case <-r.Context().Done():
			return
		}
		select {
		case response := <-request.respond:
			if response.disconnect {
				connection, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Errorf("disconnect Consul request: %v", err)
					return
				}
				_ = connection.Close()
				return
			}
			writeConsulResponse(w, "config/app", response)
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(server.Close)
	return newTestConsulProvider(t, server, "prefix"), requests
}

func writeConsulResponse(w http.ResponseWriter, key string, response consulResponse) {
	w.Header().Set("X-Consul-Index", response.index)
	if response.status != 0 && response.status != http.StatusOK {
		w.WriteHeader(response.status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	index, _ := strconv.ParseUint(response.index, 10, 64)
	raw := []byte(response.raw)
	if response.nilValue {
		raw = nil
	}
	_ = json.NewEncoder(w).Encode([]*consulapi.KVPair{{Key: key, Value: raw, ModifyIndex: index}})
}

func startConsulWatch(t *testing.T, source *ConsulProvider, requests <-chan consulRequest, initial consulResponse) <-chan provider.Event {
	t.Helper()
	result := beginConsulWatch(source)
	nextConsulRequest(t, requests).respond <- initial
	return finishConsulWatch(t, result)
}

func beginConsulWatch(source *ConsulProvider) <-chan consulWatchResult {
	result := make(chan consulWatchResult, 1)
	go func() {
		events, err := source.WatchConfig("config/app")
		result <- consulWatchResult{events, err}
	}()
	return result
}

func finishConsulWatch(t *testing.T, result <-chan consulWatchResult) <-chan provider.Event {
	t.Helper()
	select {
	case result := <-result:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.events
	case <-time.After(3 * time.Second):
		t.Fatal("WatchConfig did not return after the initial response")
		return nil
	}
}

func nextConsulRequest(t *testing.T, requests <-chan consulRequest) consulRequest {
	t.Helper()
	return nextConsulRequestWithin(t, requests, 3*time.Second)
}

func nextConsulRequestWithin(t *testing.T, requests <-chan consulRequest, timeout time.Duration) consulRequest {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(timeout):
		t.Fatal("timed out waiting for Consul request")
		return consulRequest{}
	}
}

func nextConsulEvent(t *testing.T, events <-chan provider.Event) provider.Event {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("watch closed before the expected event")
		}
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Consul event")
		return provider.Event{}
	}
}

func assertConsulValue(t *testing.T, events <-chan provider.Event, want string) {
	t.Helper()
	if event := nextConsulEvent(t, events); event.Path != "config/app" || event.Err != nil || string(event.Config) != want {
		t.Fatalf("config event = %+v, want %s", event, want)
	}
}

func assertNoConsulEvent(t *testing.T, events <-chan provider.Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected config event: %+v", event)
	default:
	}
}
