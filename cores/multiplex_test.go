package cores

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

func TestServiceAddressRejectsConflictingValues(t *testing.T) {
	managementAddr := availableTestAddr(t)
	httpAddr, grpcAddr := availableTestAddr(t), availableTestAddr(t)
	workerStarted := false
	service := NewCores(
		WithInternalHTTPAddress(managementAddr),
		WithServiceAddress(httpAddr), WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
		WithServiceAddress(grpcAddr), WithGrpcServer(newHealthServer()),
		WithWorker(func(context.Context) error { workerStarted = true; return nil }),
	)
	if err := Start(service); err == nil || !strings.Contains(err.Error(), "one business") {
		t.Fatalf("Start returned %v, want shared address error", err)
	}
	if workerStarted {
		t.Fatal("Worker started before protocol addresses were validated")
	}
	for _, addr := range []string{managementAddr, httpAddr, grpcAddr} {
		assertAddrReleased(t, addr)
	}
}

func TestMultiplexerShutdownWaitsForIdentificationTimeout(t *testing.T) {
	config := *defaultHttpServerConfig
	config.ReadHeaderTimeout = 300 * time.Millisecond
	running := startTestService(t,
		WithHttpServerConfig(&config),
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
		WithGrpcServer(newHealthServer()),
	)
	conn, err := net.DialTimeout("tcp", running.grpcAddr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(serviceTestTimeout))
	// 完成 HTTP/2 连接握手但不发送 HEADERS，确保连接已被 cmux 接收且仍在识别协议。
	if _, err := conn.Write([]byte(http2.ClientPreface)); err != nil {
		t.Fatal(err)
	}
	framer := http2.NewFramer(conn, conn)
	if err := framer.WriteSettings(); err != nil {
		t.Fatal(err)
	}
	if frame, err := framer.ReadFrame(); err != nil {
		t.Fatal(err)
	} else if _, ok := frame.(*http2.SettingsFrame); !ok {
		t.Fatalf("handshake response = %T, want SETTINGS", frame)
	}
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("idle TCP connection survived shutdown")
	}
	assertAddrReleased(t, running.grpcAddr)
}

func TestHttpHandlerGroupsValidateAndAppend(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})
	for _, options := range [][]ServiceOption{
		{WithServiceAddress("127.0.0.1:0"), WithHttpHandler()},
		{WithServiceAddress("127.0.0.1:0"), WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: handler}, HttpHandler{ApiPrefix: "api/", Handler: handler})},
		{WithServiceAddress("127.0.0.1:0"), WithHttpHandler(HttpHandler{ApiPrefix: "/api/{", Handler: handler})},
		{WithServiceAddress("127.0.0.1:0"), WithHttpHandler(HttpHandler{ApiPrefix: "/api/{id}", Handler: handler})},
	} {
		options = append(options, WithInternalHTTPAddress("127.0.0.1:0"))
		if err := Start(NewCores(options...)); err == nil {
			t.Fatal("invalid HTTP handler groups were accepted")
		}
	}
	// 旧的分次挂载习惯仍可使用，但只能追加到同一业务地址。
	running := startTestService(t,
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/first", Handler: handler}),
		WithHttpHandler(HttpHandler{ApiPrefix: "/second", Handler: handler}),
	)
	assertHttpResponse(t, running.businessAddrs[0], "/first/hello", http.StatusOK, "/hello")
	assertHttpResponse(t, running.businessAddrs[0], "/second/hello", http.StatusOK, "/hello")
}

func TestBusinessHandlersRequireServiceAddress(t *testing.T) {
	for _, options := range [][]ServiceOption{
		{WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()})},
		{WithGrpcServer(newHealthServer())},
		{WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}), WithGrpcServer(newHealthServer())},
	} {
		managementAddr := availableTestAddr(t)
		options = append(options, WithInternalHTTPAddress(managementAddr))
		if err := Start(NewCores(options...)); err == nil || !strings.Contains(err.Error(), "business listen address") {
			t.Fatalf("Start returned %v, want missing business address error", err)
		}
		assertAddrReleased(t, managementAddr)
	}
}
