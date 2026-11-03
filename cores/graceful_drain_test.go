package cores

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestWaitAllDoneDrainsHttpRequests(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("SSE=%t", streaming), func(t *testing.T) {
			oldMaxWait := defaultMaxWait
			t.Cleanup(func() { defaultMaxWait = oldMaxWait })
			entered, canceled, lateRequest := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
			release := make(chan struct{}, 1)
			running := startTestService(t,
				WithWaitAllDone(),
				WithDefaultMaxWait(40*time.Millisecond),
				WithServiceAddress("127.0.0.1:0"),
				WithGrpcServer(newHealthServer()),
				WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/late" {
						lateRequest <- struct{}{}
						return
					}
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, "data: started\n\n")
						w.(http.Flusher).Flush()
					}
					close(entered)
					select {
					case <-release:
						if streaming {
							_, _ = io.WriteString(w, "data: done\n\n")
						} else {
							w.Header().Set("Content-Type", "application/json")
							_, _ = io.WriteString(w, `{"done":true}`)
						}
					case <-r.Context().Done():
						close(canceled)
					}
				})}),
			)
			t.Cleanup(func() { close(release) })
			client := &http.Client{Timeout: serviceTestTimeout}
			t.Cleanup(client.CloseIdleConnections)
			requestDone := make(chan error, 1)
			go func() {
				response, err := client.Get("http://" + running.businessAddrs[0] + "/api/wait")
				if err == nil {
					defer response.Body.Close()
					var body []byte
					body, err = io.ReadAll(response.Body)
					want := `{"done":true}`
					if streaming {
						want = "data: started\n\ndata: done\n\n"
					}
					if err == nil && (response.StatusCode != http.StatusOK || string(body) != want) {
						err = fmt.Errorf("response = %d %q, want 200 %q", response.StatusCode, body, want)
					}
				}
				requestDone <- err
			}()
			select {
			case <-entered:
			case <-time.After(serviceTestTimeout):
				t.Fatal("request did not enter handler")
			}
			running.service.cancel()
			select {
			case <-canceled:
				t.Fatal("shutdown canceled the in-flight request context")
			case <-running.finished:
				t.Fatalf("service exited before the request finished: %v", running.err)
			case err := <-requestDone:
				t.Fatalf("request stopped before its handler was released: %v", err)
			case <-time.After(120 * time.Millisecond):
			}
			lateCtx, cancelLate := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancelLate()
			request, err := http.NewRequestWithContext(lateCtx, http.MethodGet, "http://"+running.businessAddrs[0]+"/api/late", nil)
			if err != nil {
				t.Fatal(err)
			}
			if response, err := client.Do(request); err == nil {
				_ = response.Body.Close()
			}
			select {
			case <-lateRequest:
				t.Fatal("a new request entered its handler while the service was draining")
			default:
			}
			release <- struct{}{}
			select {
			case err := <-requestDone:
				if err != nil {
					t.Fatalf("in-flight request failed: %v", err)
				}
			case <-time.After(serviceTestTimeout):
				t.Fatal("request did not complete")
			}
			if err := running.wait(t); err != nil {
				t.Fatalf("Start returned %v", err)
			}
			assertAddrReleased(t, running.businessAddrs[0])
		})
	}
}

func TestWaitAllDonePreservesHttpClientCancellation(t *testing.T) {
	oldMaxWait := defaultMaxWait
	t.Cleanup(func() { defaultMaxWait = oldMaxWait })
	canceled := make(chan struct{})
	running := startTestService(t,
		WithWaitAllDone(),
		WithDefaultMaxWait(40*time.Millisecond),
		WithServiceAddress("127.0.0.1:0"),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: started\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(canceled)
		})}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+running.businessAddrs[0]+"/api/", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	running.service.cancel()
	select {
	case <-canceled:
		t.Fatal("shutdown canceled the request before its client disconnected")
	case <-running.finished:
		t.Fatalf("service exited with an active stream: %v", running.err)
	case <-time.After(120 * time.Millisecond):
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(serviceTestTimeout):
		t.Fatal("handler did not observe its client's cancellation")
	}
	if err := running.wait(t); err != nil {
		t.Fatalf("Start returned %v", err)
	}
}

type drainingHealthStream struct {
	healthpb.UnimplementedHealthServer
	release, canceled chan struct{}
}

func (server *drainingHealthStream) Watch(_ *healthpb.HealthCheckRequest, stream grpc.ServerStreamingServer[healthpb.HealthCheckResponse]) error {
	if err := stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}); err != nil {
		return err
	}
	select {
	case <-server.release:
		return stream.Send(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_NOT_SERVING})
	case <-stream.Context().Done():
		close(server.canceled)
		return stream.Context().Err()
	}
}

func TestWaitAllDoneDrainsGrpcStream(t *testing.T) {
	oldMaxWait := defaultMaxWait
	t.Cleanup(func() { defaultMaxWait = oldMaxWait })
	handler := &drainingHealthStream{release: make(chan struct{}, 1), canceled: make(chan struct{})}
	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, handler)
	running := startTestService(t,
		WithWaitAllDone(),
		WithDefaultMaxWait(40*time.Millisecond),
		WithServiceAddress("127.0.0.1:0"),
		WithGrpcServer(server),
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
	)
	t.Cleanup(func() { close(handler.release) })
	ctx, cancel := context.WithTimeout(context.Background(), serviceTestTimeout)
	defer cancel()
	stream, err := healthClient(t, running.grpcAddr).Watch(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil {
		t.Fatal(err)
	}
	if response, err := stream.Recv(); err != nil || response.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("initial response = %v, error %v", response, err)
	}
	running.service.cancel()
	select {
	case <-handler.canceled:
		t.Fatal("shutdown canceled the in-flight gRPC stream")
	case <-running.finished:
		t.Fatalf("service exited before the gRPC stream finished: %v", running.err)
	case <-time.After(120 * time.Millisecond):
	}
	handler.release <- struct{}{}
	if response, err := stream.Recv(); err != nil || response.Status != healthpb.HealthCheckResponse_NOT_SERVING {
		t.Fatalf("final response = %v, error %v", response, err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("stream finished with %v, want EOF", err)
	}
	if err := running.wait(t); err != nil {
		t.Fatalf("Start returned %v", err)
	}
}
