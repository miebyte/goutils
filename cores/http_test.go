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
	running := startTestService(t,
		WithPprof(),
		WithSwagger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "swagger")
		})),
		WithServiceAddress(addr),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})}),
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

func TestStartHttpWithoutWorkers(t *testing.T) {
	addr := availableTestAddr(t)
	service := NewCores(
		WithInternalHTTPAddress("127.0.0.1:0"),
		WithPprof(),
		WithServiceAddress(addr),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})}),
	)
	running := runTestService(t, service, true)
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
		t.Fatalf("Start returned %v", err)
	}
	assertAddrReleased(t, addr)
}

func TestStartHttpOutlivesShortWorker(t *testing.T) {
	workerDone := make(chan struct{})
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "running")
		})}),
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
	assertHttpResponse(t, running.httpAddr, "/ready", http.StatusOK, "ok")
	select {
	case <-running.finished:
		t.Fatalf("HTTP service exited when its short Workers finished: %v", running.err)
	default:
	}
}

func TestStartBusinessHttpAndGrpc(t *testing.T) {
	running := startTestService(t,
		WithPprof(),
		WithServiceAddress("127.0.0.1:0"),
		WithGrpcServer(newHealthServer()),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})}),
	)
	checkHealth(t, healthClient(t, running.grpcAddr))
	assertHttpResponse(t, running.businessAddrs[0], "/api/hello", http.StatusOK, "/hello")
	assertHttpResponse(t, running.businessAddrs[0], "/health_check", http.StatusNotFound, "")
	assertHttpResponse(t, running.businessAddrs[0], "/debug/pprof/", http.StatusNotFound, "")
	assertHttpResponse(t, running.httpAddr, "/ready", http.StatusOK, "ok")
}

func TestHttpHandlersShareAddress(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path)
	})
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "first", Handler: handler}, HttpHandler{ApiPrefix: "/second/", Handler: handler}),
	)
	if len(running.businessAddrs) != 1 {
		t.Fatalf("business listeners = %d, want 1 for handlers sharing an address", len(running.businessAddrs))
	}
	for _, path := range []string{"/first/hello", "/second/hello"} {
		assertHttpResponse(t, running.businessAddrs[0], path, http.StatusOK, "/hello")
	}
}

func TestHttpHandlersRejectConflictingServiceAddresses(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "running")
	})
	firstAddr, secondAddr, managementAddr := availableTestAddr(t), availableTestAddr(t), availableTestAddr(t)
	workerStarted := false
	service := NewCores(
		WithInternalHTTPAddress(managementAddr),
		WithServiceAddress(firstAddr),
		WithHttpHandler(HttpHandler{ApiPrefix: "/first", Handler: handler}),
		WithServiceAddress(secondAddr),
		WithHttpHandler(HttpHandler{ApiPrefix: "/second", Handler: handler}),
		WithWorker(func(context.Context) error { workerStarted = true; return nil }),
	)
	if err := Start(service); err == nil || !strings.Contains(err.Error(), "one business") {
		t.Fatalf("Start returned %v, want single HTTP address error", err)
	}
	if workerStarted {
		t.Fatal("Worker started with invalid HTTP configuration")
	}
	for _, addr := range []string{firstAddr, secondAddr, managementAddr} {
		assertAddrReleased(t, addr)
	}
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
			err := Start(NewCores(WithInternalHTTPAddress("127.0.0.1:0"), WithServiceAddress(tt.addr), WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: tt.handler})))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("Start returned %v, want %s configuration error", err, tt.want)
			}
		})
	}
}

func TestHttpListenFailureRollsBackListeners(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	internalAddr := availableTestAddr(t)
	workerStarted := make(chan struct{}, 1)
	service := NewCores(
		WithInternalHTTPAddress(internalAddr),
		WithServiceAddress(occupied.Addr().String()),
		WithHttpHandler(HttpHandler{ApiPrefix: "/second", Handler: http.NotFoundHandler()}),
		WithWorker(func(context.Context) error {
			workerStarted <- struct{}{}
			return nil
		}),
	)
	if err := Start(service); err == nil {
		t.Fatal("Start succeeded with an occupied business HTTP port")
	}
	select {
	case <-workerStarted:
		t.Fatal("Worker ran before all listeners were bound")
	default:
	}
	assertAddrReleased(t, internalAddr)
}

func TestHttpShutdownWaitsForInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithGrpcServer(newHealthServer()),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "done")
		})}),
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
		t.Fatalf("Start returned %v", err)
	}
	assertAddrReleased(t, running.businessAddrs[0])
}

func TestHttpShutdownForcesLongRequestToStop(t *testing.T) {
	oldMaxWait := defaultMaxWait
	t.Cleanup(func() { defaultMaxWait = oldMaxWait })
	started := make(chan struct{})
	running := startTestService(t,
		WithDefaultMaxWait(100*time.Millisecond),
		WithServiceAddress("127.0.0.1:0"),
		WithGrpcServer(newHealthServer()),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		})}),
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
		t.Fatalf("Start returned %v, want shutdown deadline error", err)
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
