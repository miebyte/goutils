package cores

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHttpPortsAreIsolated(t *testing.T) {
	addr := availableTestAddr(t)
	running := startTestService(t, true,
		WithPprof(),
		WithSwagger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "swagger")
		})),
		WithHttpHandler(addr, "/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})),
	)
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	for _, tt := range []struct {
		name   string
		addr   string
		path   string
		status int
		body   string
	}{
		{name: "business API", addr: addr, path: "/api/hello", status: http.StatusOK, body: "/hello"},
		{name: "internal health", addr: running.httpAddr, path: "/health_check", status: http.StatusOK, body: "ok"},
		{name: "internal pprof", addr: running.httpAddr, path: "/debug/pprof/", status: http.StatusOK},
		{name: "internal swagger", addr: running.httpAddr, path: "/docs/index.html", status: http.StatusOK, body: "swagger"},
		{name: "business health excluded", addr: addr, path: "/health_check", status: http.StatusNotFound},
		{name: "business pprof excluded", addr: addr, path: "/debug/pprof/", status: http.StatusNotFound},
		{name: "business swagger excluded", addr: addr, path: "/docs/index.html", status: http.StatusNotFound},
		{name: "internal business API excluded", addr: running.httpAddr, path: "/api/hello", status: http.StatusNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.Get("http://" + tt.addr + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tt.status || (tt.body != "" && string(body) != tt.body) {
				t.Fatalf("GET %s%s = %d %q, want %d %q", tt.addr, tt.path, response.StatusCode, body, tt.status, tt.body)
			}
		})
	}
}

func assertHttpResponse(t *testing.T, addr, path string, status int, wantBody string) {
	t.Helper()
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Get("http://" + addr + path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status || (wantBody != "" && string(body) != wantBody) {
		t.Fatalf("GET %s%s = %d %q, want %d %q", addr, path, response.StatusCode, body, status, wantBody)
	}
}

func TestRunHttpWithoutWorkers(t *testing.T) {
	addr := availableTestAddr(t)
	service := NewCores(
		WithPprof(),
		WithHttpHandler(addr, "/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})),
	)
	running := runTestService(t, service, "")
	deadline := time.NewTimer(serviceTestTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		select {
		case <-running.finished:
			t.Fatalf("HTTP service exited without Workers: %v", running.err)
		case <-deadline.C:
			t.Fatal("HTTP service did not listen")
		case <-ticker.C:
		}
	}
	assertHttpResponse(t, addr, "/api/hello", http.StatusOK, "/hello")
	assertHttpResponse(t, addr, "/health_check", http.StatusNotFound, "")
	assertHttpResponse(t, addr, "/debug/pprof/", http.StatusNotFound, "")
	service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	assertAddrReleased(t, addr)
}

func TestRunHttpOutlivesShortWorker(t *testing.T) {
	workerDone := make(chan struct{})
	running := startTestService(t, false,
		WithHttpHandler("127.0.0.1:0", "/api", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "running")
		})),
		WithWorker(func(context.Context) error {
			close(workerDone)
			return nil
		}),
	)
	select {
	case <-workerDone:
	case <-time.After(serviceTestTimeout):
		t.Fatal("short Worker did not finish")
	}
	assertHttpResponse(t, running.businessAddrs[0], "/api/", http.StatusOK, "running")
	if running.httpAddr != "" {
		t.Fatalf("Run opened an internal HTTP listener at %s", running.httpAddr)
	}
	select {
	case <-running.finished:
		t.Fatalf("HTTP service exited when its short Workers finished: %v", running.err)
	default:
	}
}

func TestRunHttpAndGrpc(t *testing.T) {
	running := startTestService(t, false,
		WithPprof(),
		WithGrpcServer("127.0.0.1:0", newHealthServer()),
		WithHttpHandler("127.0.0.1:0", "/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})),
	)
	checkHealth(t, healthClient(t, running.grpcAddr))
	assertHttpResponse(t, running.businessAddrs[0], "/api/hello", http.StatusOK, "/hello")
	assertHttpResponse(t, running.businessAddrs[0], "/health_check", http.StatusNotFound, "")
	assertHttpResponse(t, running.businessAddrs[0], "/debug/pprof/", http.StatusNotFound, "")
	if running.httpAddr != "" {
		t.Fatalf("Run opened an internal HTTP listener at %s", running.httpAddr)
	}
}

func TestHttpHandlersShareAddress(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path)
	})
	running := startTestService(t, false,
		WithHttpHandler("127.0.0.1:0", "first", handler),
		WithHttpHandler("127.0.0.1:0", "/second/", handler),
	)
	if len(running.businessAddrs) != 1 {
		t.Fatalf("business listeners = %d, want 1 for handlers sharing an address", len(running.businessAddrs))
	}
	for _, path := range []string{"/first/hello", "/second/hello"} {
		assertHttpResponse(t, running.businessAddrs[0], path, http.StatusOK, "/hello")
	}
}

func TestHttpHandlersUseSeparateAddresses(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "running")
	})
	running := startTestService(t, false,
		WithHttpHandler("127.0.0.1:0", "/first", handler),
		WithHttpHandler(availableTestAddr(t), "/second", handler),
	)
	if len(running.businessAddrs) != 2 {
		t.Fatalf("business listeners = %d, want 2", len(running.businessAddrs))
	}
	assertHttpResponse(t, running.businessAddrs[0], "/first/hello", http.StatusOK, "running")
	assertHttpResponse(t, running.businessAddrs[0], "/second/hello", http.StatusNotFound, "")
	assertHttpResponse(t, running.businessAddrs[1], "/second/hello", http.StatusOK, "running")
	assertHttpResponse(t, running.businessAddrs[1], "/first/hello", http.StatusNotFound, "")
}

func TestHttpConfigurationErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		addr    string
		handler http.Handler
		want    string
	}{
		{name: "nil handler", addr: "127.0.0.1:0", want: "handler"},
		{name: "empty address", handler: http.NotFoundHandler(), want: "address"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(NewCores(WithHttpHandler(tt.addr, "/api", tt.handler)))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("Run returned %v, want %s configuration error", err, tt.want)
			}
		})
	}
}

func TestHttpListenFailureRollsBackListeners(t *testing.T) {
	for _, mode := range []string{"Run", "Start"} {
		t.Run(mode, func(t *testing.T) {
			occupied, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer occupied.Close()
			businessAddr := availableTestAddr(t)
			internalAddr := availableTestAddr(t)
			workerStarted := make(chan struct{}, 1)
			service := NewCores(
				WithHttpHandler(businessAddr, "/first", http.NotFoundHandler()),
				WithHttpHandler(occupied.Addr().String(), "/second", http.NotFoundHandler()),
				WithWorker(func(context.Context) error {
					workerStarted <- struct{}{}
					return nil
				}),
			)
			if mode == "Run" {
				err = Run(service)
			} else {
				err = Start(service, internalAddr)
			}
			if err == nil {
				t.Fatalf("%s succeeded with an occupied business HTTP port", mode)
			}
			select {
			case <-workerStarted:
				t.Fatal("Worker ran before all listeners were bound")
			default:
			}
			assertAddrReleased(t, businessAddr)
			assertAddrReleased(t, internalAddr)
		})
	}
}

func TestHttpShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	running := startTestService(t, false,
		WithHttpHandler("127.0.0.1:0", "/api", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "done")
		})),
	)
	defer close(release)
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	requestDone := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + running.businessAddrs[0] + "/api/")
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			err = readErr
			if err == nil && (response.StatusCode != http.StatusOK || string(body) != "done") {
				err = errors.New("in-flight HTTP response was incomplete")
			}
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(serviceTestTimeout):
		t.Fatal("HTTP request did not enter handler")
	}
	running.service.cancel()
	select {
	case <-running.finished:
		t.Fatalf("service exited before the in-flight HTTP request finished: %v", running.err)
	case <-time.After(50 * time.Millisecond):
	}
	release <- struct{}{}
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatalf("in-flight HTTP request failed during graceful shutdown: %v", err)
		}
	case <-time.After(serviceTestTimeout):
		t.Fatal("HTTP request did not finish")
	}
	if err := running.wait(t); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	assertAddrReleased(t, running.businessAddrs[0])
}

func TestHttpShutdownForcesLongRequestToStop(t *testing.T) {
	oldMaxWait := defaultMaxWait
	t.Cleanup(func() { defaultMaxWait = oldMaxWait })
	started := make(chan struct{})
	running := startTestService(t, false,
		WithDefaultMaxWait(100*time.Millisecond),
		WithHttpHandler("127.0.0.1:0", "/api", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		})),
	)
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	requestDone := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + running.businessAddrs[0] + "/api/")
		if err == nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(serviceTestTimeout):
		t.Fatal("HTTP request did not enter handler")
	}
	shutdownStarted := time.Now()
	running.service.cancel()
	if err := running.wait(t); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run returned %v, want shutdown deadline error", err)
	}
	if elapsed := time.Since(shutdownStarted); elapsed > 2*time.Second {
		t.Fatalf("forced shutdown took %v", elapsed)
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("long HTTP request survived forced shutdown")
		}
	case <-time.After(serviceTestTimeout):
		t.Fatal("HTTP client did not observe forced shutdown")
	}
	assertAddrReleased(t, running.businessAddrs[0])
}
