package cores

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRunWorkersWithoutServers(t *testing.T) {
	for _, wantErr := range []error{nil, errors.New("worker failed")} {
		service := NewCores(WithWorker(func(context.Context) error { return wantErr }))
		running := runTestService(t, service, false)
		if err := running.wait(t); !errors.Is(err, wantErr) {
			t.Fatalf("Run returned %v, want %v", err, wantErr)
		}
	}
}

func TestStartHttpWithoutGrpc(t *testing.T) {
	running := startTestService(t)
	client := &http.Client{Timeout: serviceTestTimeout}
	t.Cleanup(client.CloseIdleConnections)
	response, err := client.Get("http://" + running.httpAddr + "/health_check")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	running.service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	assertAddrReleased(t, running.httpAddr)
}

func TestStartCronWithoutBusinessServers(t *testing.T) {
	jobRan := make(chan struct{}, 1)
	service := NewCores(WithInternalHTTPAddress("127.0.0.1:0"), WithCronWorker("test-cron", "@every 1s", func(context.Context) error {
		select {
		case jobRan <- struct{}{}:
		default:
		}
		return nil
	}))
	running := runTestService(t, service, true)
	select {
	case <-jobRan:
	case <-running.finished:
		t.Fatalf("Cron service exited before its job ran: %v", running.err)
	case <-time.After(serviceTestTimeout):
		t.Fatal("Cron job did not run")
	}
	service.cancel()
	if err := running.wait(t); err != nil {
		t.Fatalf("Start returned %v", err)
	}
}
