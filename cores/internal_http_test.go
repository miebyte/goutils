package cores

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStartDefaultManagementPort(t *testing.T) {
	// 只检查测试机器是否已被其他程序占用，不改变默认端口的生产行为。
	probe, err := net.Listen("tcp", ":6060")
	if err != nil {
		t.Skipf("default management port is already in use: %v", err)
	}
	_ = probe.Close()
	started := make(chan struct{}, 1)
	service := NewCores(WithWorker(func(context.Context) error {
		started <- struct{}{}
		return nil
	}))
	running := runTestService(t, service, true)
	select {
	case <-started:
	case <-running.finished:
		t.Fatalf("Start returned before listening: %v", running.err)
	case <-time.After(serviceTestTimeout):
		t.Fatal("Start did not initialize the default management listener")
	}
	assertHttpResponse(t, "127.0.0.1:6060", "/health_check", http.StatusOK, "ok")
	assertHttpResponse(t, "127.0.0.1:6060", "/ready", http.StatusOK, "ok")
	// 服务模式的短 Worker 完成后仍保留管理服务，等待显式停机。
	select {
	case <-running.finished:
		t.Fatalf("service exited when its Worker finished: %v", running.err)
	default:
	}
	service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatal(err)
	}
	assertAddrReleased(t, ":6060")
}

func TestEmptyManagementAddressStopsStartup(t *testing.T) {
	// 空地址会让 net.Listen 随机分配端口，管理服务不能静默采用这种回退。
	err := Start(NewCores(WithInternalHTTPAddress("")))
	if err == nil || !strings.Contains(err.Error(), "address must not be empty") {
		t.Fatalf("Start returned %v, want management address error", err)
	}
}

func TestInternalHTTPPortConflictStopsStartup(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	workerStarted := false
	service := NewCores(
		WithInternalHTTPAddress(occupied.Addr().String()),
		WithWorker(func(context.Context) error {
			workerStarted = true
			return nil
		}),
	)
	if err := Start(service); err == nil {
		t.Fatal("Start succeeded despite the management port conflict")
	}
	if workerStarted {
		t.Fatal("Worker started without the mandatory management listener")
	}
}

func TestRunRejectsBusinessListeners(t *testing.T) {
	for _, option := range []ServiceOption{
		WithHttpHandler(HttpHandler{ApiPrefix: "/api", Handler: http.NotFoundHandler()}),
		WithGrpcServer(newHealthServer()),
	} {
		service := NewCores(option)
		err := Run(service)
		if err == nil || !strings.Contains(err.Error(), "script mode") {
			t.Fatalf("Run returned %v, want script mode error", err)
		}
		if service.internalHTTP.listener != nil {
			t.Fatal("script opened a management listener")
		}
	}
}
