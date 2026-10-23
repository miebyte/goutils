package cores

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const serviceTestTimeout = 5 * time.Second

type testService struct {
	service       *CoresService
	finished      chan struct{}
	err           error
	grpcAddr      string
	httpAddr      string
	businessAddrs []string
}

func runTestService(t *testing.T, service *CoresService, httpAddr string) *testService {
	t.Helper()
	running := &testService{service: service, finished: make(chan struct{})}
	go func() {
		if httpAddr == "" {
			running.err = Run(service)
		} else {
			running.err = Start(service, httpAddr)
		}
		close(running.finished)
	}()
	t.Cleanup(func() {
		service.cancel()
		running.wait(t)
	})
	return running
}

func startTestService(t *testing.T, mixed bool, opts ...ServiceOption) *testService {
	t.Helper()
	type addresses struct {
		grpc, http string
		business   []string
	}
	ready := make(chan addresses, 1)
	var service *CoresService
	// A Worker starts after all listeners have been initialized. Publishing the
	// addresses here keeps the tests from racing with service startup.
	opts = append(opts, WithNameWorker("test-listener-addresses", func(context.Context) error {
		var addr addresses
		if service.grpc != nil {
			addr.grpc = service.grpc.listener.Addr().String()
		}
		if service.internalHTTP.listener != nil {
			addr.http = service.internalHTTP.listener.Addr().String()
		}
		for _, server := range service.httpServers {
			addr.business = append(addr.business, server.listener.Addr().String())
		}
		ready <- addr
		return nil
	}))
	service = NewCores(opts...)
	httpAddr := ""
	if mixed {
		httpAddr = "127.0.0.1:0"
	}
	running := runTestService(t, service, httpAddr)
	select {
	case addr := <-ready:
		running.grpcAddr, running.httpAddr = addr.grpc, addr.http
		running.businessAddrs = addr.business
	case <-running.finished:
		t.Fatalf("service exited during startup: %v", running.err)
	case <-time.After(serviceTestTimeout):
		t.Fatal("service did not initialize listeners")
	}
	return running
}

func (running *testService) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-running.finished:
		return running.err
	case <-time.After(serviceTestTimeout):
		t.Fatal("service did not stop")
		return nil
	}
}

func newHealthServer() *grpc.Server {
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, health.NewServer())
	return server
}

func healthClient(t *testing.T, addr string) healthpb.HealthClient {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

func checkHealth(t *testing.T, client healthpb.HealthClient) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
	defer cancel()
	response, err := client.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatalf("health RPC failed: %v", err)
	}
	if response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %v, want SERVING", response.Status)
	}
}

func availableTestAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func assertAddrReleased(t *testing.T, addr string) {
	t.Helper()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("address %s was not released: %v", addr, err)
	}
	_ = listener.Close()
}

func TestRunGrpcWithoutWorkers(t *testing.T) {
	addr := availableTestAddr(t)
	service := NewCores(WithGrpcServer(addr, newHealthServer()))
	running := runTestService(t, service, "")
	checkHealth(t, healthClient(t, addr))
	select {
	case <-running.finished:
		t.Fatalf("gRPC service exited without Workers: %v", running.err)
	default:
	}
	service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	assertAddrReleased(t, addr)
}

func TestRunGrpcOutlivesShortWorker(t *testing.T) {
	workerDone := make(chan struct{})
	running := startTestService(t, false,
		WithGrpcServer("127.0.0.1:0", newHealthServer()),
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
	checkHealth(t, healthClient(t, running.grpcAddr))
	select {
	case <-running.finished:
		t.Fatalf("service exited when its short Workers finished: %v", running.err)
	default:
	}
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestStartHttpAndGrpc(t *testing.T) {
	running := startTestService(t, true,
		WithGrpcServer("127.0.0.1:0", newHealthServer()),
		WithHttpHandler("127.0.0.1:0", "/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.URL.Path)
		})),
	)
	checkHealth(t, healthClient(t, running.grpcAddr))
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	for _, tt := range []struct {
		addr string
		path string
		want string
	}{
		{addr: running.httpAddr, path: "/health_check", want: "ok"},
		{addr: running.businessAddrs[0], path: "/api/hello", want: "/hello"},
	} {
		response, err := client.Get("http://" + tt.addr + tt.path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK || string(body) != tt.want {
			t.Fatalf("GET %s = %d %q, want 200 %q", tt.path, response.StatusCode, body, tt.want)
		}
	}
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	assertAddrReleased(t, running.httpAddr)
	assertAddrReleased(t, running.businessAddrs[0])
	assertAddrReleased(t, running.grpcAddr)
}

func TestGrpcConfigurationErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		addr   string
		server *grpc.Server
		want   string
	}{
		{name: "nil server", addr: "127.0.0.1:0", want: "server"},
		{name: "empty address", server: grpc.NewServer(), want: "address"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(NewCores(WithGrpcServer(tt.addr, tt.server)))
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("Run returned %v, want %s configuration error", err, tt.want)
			}
		})
	}
}

func TestGrpcListenFailureRollsBackHttp(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	httpAddr := availableTestAddr(t)
	businessAddr := availableTestAddr(t)
	workerStarted := make(chan struct{}, 1)
	service := NewCores(
		WithGrpcServer(occupied.Addr().String(), newHealthServer()),
		WithHttpHandler(businessAddr, "/api", http.NotFoundHandler()),
		WithWorker(func(context.Context) error {
			workerStarted <- struct{}{}
			return nil
		}),
	)
	if err := Start(service, httpAddr); err == nil {
		t.Fatal("Start succeeded with an occupied gRPC port")
	}
	select {
	case <-workerStarted:
		t.Fatal("Worker ran before all listeners were bound")
	default:
	}
	assertAddrReleased(t, httpAddr)
	assertAddrReleased(t, businessAddr)
}

func TestServeErrorStopsHttpGrpcAndWorkers(t *testing.T) {
	for _, protocol := range []string{"grpc", "internal http", "business http"} {
		t.Run(protocol, func(t *testing.T) {
			workerStopped := make(chan struct{})
			running := startTestService(t, true,
				WithGrpcServer("127.0.0.1:0", newHealthServer()),
				WithHttpHandler("127.0.0.1:0", "/api", http.NotFoundHandler()),
				WithHttpHandler(availableTestAddr(t), "/other", http.NotFoundHandler()),
				WithWorker(func(ctx context.Context) error {
					<-ctx.Done()
					close(workerStopped)
					return ctx.Err()
				}),
			)
			checkHealth(t, healthClient(t, running.grpcAddr))
			listener := running.service.grpc.listener
			if protocol == "internal http" {
				listener = running.service.internalHTTP.listener
			} else if protocol == "business http" {
				listener = running.service.httpServers[0].listener
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			if err := running.wait(t); !errors.Is(err, net.ErrClosed) {
				t.Fatalf("Start returned %v, want the listener error", err)
			}
			select {
			case <-workerStopped:
			default:
				t.Fatal("service returned before Worker stopped")
			}
			assertAddrReleased(t, running.httpAddr)
			for _, addr := range running.businessAddrs {
				assertAddrReleased(t, addr)
			}
			assertAddrReleased(t, running.grpcAddr)
		})
	}
}

func TestWorkerErrorStopsHttpAndGrpc(t *testing.T) {
	fail := make(chan struct{})
	wantErr := errors.New("worker failed")
	running := startTestService(t, true,
		WithGrpcServer("127.0.0.1:0", newHealthServer()),
		WithHttpHandler("127.0.0.1:0", "/api", http.NotFoundHandler()),
		WithWorker(func(context.Context) error {
			<-fail
			return wantErr
		}),
	)
	checkHealth(t, healthClient(t, running.grpcAddr))
	close(fail)
	if err := running.wait(t); !errors.Is(err, wantErr) {
		t.Fatalf("Start returned %v, want Worker error", err)
	}
	assertAddrReleased(t, running.httpAddr)
	assertAddrReleased(t, running.businessAddrs[0])
	assertAddrReleased(t, running.grpcAddr)
}

type blockingHealthServer struct {
	healthpb.UnimplementedHealthServer
	started chan struct{}
	release chan struct{}
}

func (server *blockingHealthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	close(server.started)
	select {
	case <-server.release:
		return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (server *blockingHealthServer) Watch(_ *healthpb.HealthCheckRequest, stream grpc.ServerStreamingServer[healthpb.HealthCheckResponse]) error {
	if err := stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}); err != nil {
		return err
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

func TestGrpcShutdownWaitsForInFlightRPC(t *testing.T) {
	healthServer := &blockingHealthServer{started: make(chan struct{}), release: make(chan struct{})}
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	running := startTestService(t, false, WithGrpcServer("127.0.0.1:0", server))
	client := healthClient(t, running.grpcAddr)
	ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
	defer cancel()
	rpcDone := make(chan error, 1)
	go func() {
		_, err := client.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
		rpcDone <- err
	}()
	select {
	case <-healthServer.started:
	case <-ctx.Done():
		t.Fatal("RPC did not enter handler")
	}
	running.service.cancel()
	select {
	case <-running.finished:
		t.Fatalf("service exited before the in-flight RPC finished: %v", running.err)
	case <-time.After(50 * time.Millisecond):
	}
	close(healthServer.release)
	select {
	case err := <-rpcDone:
		if err != nil {
			t.Fatalf("in-flight RPC failed during graceful shutdown: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("RPC did not finish")
	}
	if err := running.wait(t); err != nil {
		t.Fatalf("Run returned %v", err)
	}
}

func TestGrpcShutdownForcesLongStreamToStop(t *testing.T) {
	oldMaxWait := defaultMaxWait
	t.Cleanup(func() { defaultMaxWait = oldMaxWait })
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, &blockingHealthServer{})
	running := startTestService(t, false,
		WithDefaultMaxWait(100*time.Millisecond),
		WithGrpcServer("127.0.0.1:0", server),
	)
	ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
	defer cancel()
	stream, err := healthClient(t, running.grpcAddr).Watch(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("stream did not start: %v", err)
	}
	started := time.Now()
	running.service.cancel()
	if err := running.wait(t); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run returned %v, want shutdown deadline error", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("forced shutdown took %v", elapsed)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("stream survived forced shutdown")
	}
	assertAddrReleased(t, running.grpcAddr)
}

type nonCooperativeHealthServer struct {
	healthpb.UnimplementedHealthServer
	started  chan struct{}
	canceled chan struct{}
	release  chan struct{}
}

func (server *nonCooperativeHealthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	close(server.started)
	<-ctx.Done()
	close(server.canceled)
	<-server.release
	return nil, ctx.Err()
}

func TestGrpcShutdownBoundsUncooperativeHandler(t *testing.T) {
	for _, waitForHandlers := range []bool{false, true} {
		t.Run(fmt.Sprintf("waitForHandlers=%t", waitForHandlers), func(t *testing.T) {
			oldMaxWait := defaultMaxWait
			t.Cleanup(func() { defaultMaxWait = oldMaxWait })
			healthServer := &nonCooperativeHealthServer{
				started:  make(chan struct{}),
				canceled: make(chan struct{}),
				release:  make(chan struct{}),
			}
			server := grpc.NewServer(grpc.WaitForHandlers(waitForHandlers))
			healthpb.RegisterHealthServer(server, healthServer)
			fail := make(chan struct{})
			workerContexts := make(chan context.Context, 1)
			workerErr := errors.New("worker failed")
			running := startTestService(t, false,
				WithDefaultMaxWait(100*time.Millisecond),
				WithGrpcServer("127.0.0.1:0", server),
				WithWorker(func(context.Context) error {
					<-fail
					return workerErr
				}),
				WithWorker(func(ctx context.Context) error {
					workerContexts <- ctx
					<-ctx.Done()
					return ctx.Err()
				}),
			)
			// Release the handler before the service cleanup waits, including when
			// the deadline assertion fails against an unbounded shutdown.
			t.Cleanup(func() { close(healthServer.release) })
			conn, err := grpc.NewClient(running.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = conn.Close() })
			ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
			defer cancel()
			rpcDone := make(chan error, 1)
			go func() {
				_, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
				rpcDone <- err
			}()
			select {
			case <-healthServer.started:
			case <-ctx.Done():
				t.Fatal("RPC did not enter handler")
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-healthServer.canceled:
			case <-ctx.Done():
				t.Fatal("server did not observe the disconnected client")
			}
			select {
			case err := <-rpcDone:
				if err == nil {
					t.Fatal("RPC succeeded after its connection was closed")
				}
			case <-ctx.Done():
				t.Fatal("client RPC did not stop after its connection was closed")
			}
			workerCtx := <-workerContexts
			close(fail)
			select {
			case <-workerCtx.Done():
				t.Fatal("Worker context was canceled before the shutdown grace period elapsed")
			case <-time.After(25 * time.Millisecond):
			}
			select {
			case <-running.finished:
				if !errors.Is(running.err, context.DeadlineExceeded) {
					t.Errorf("Run returned %v, want shutdown deadline error", running.err)
				}
				if !errors.Is(running.err, workerErr) {
					t.Errorf("Run returned %v, want Worker error", running.err)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("Run did not return at its shutdown deadline while the handler remained blocked")
			}
		})
	}
}
