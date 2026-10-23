package cores

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestGracefulSignalsReturnSuccess(t *testing.T) {
	if mode := os.Getenv("GOUTILS_GRACEFUL_TEST_MODE"); mode != "" {
		var stopSignal os.Signal = os.Interrupt
		if os.Getenv("GOUTILS_GRACEFUL_TEST_SIGNAL") == "terminate" {
			stopSignal = syscall.SIGTERM
		}
		// 信号只发给隔离的测试子进程。保留保护订阅，避免服务尚未安装监听时触发 OS 默认退出。
		guard := make(chan os.Signal, 1)
		signal.Notify(guard, stopSignal)
		defer signal.Stop(guard)
		process, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		defer process.Release()
		senderCtx, stopSender := context.WithCancel(context.Background())
		senderDone := make(chan struct{})
		go func() {
			defer close(senderDone)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-senderCtx.Done():
					return
				case <-ticker.C:
					// 重发用于跨过监听安装窗口；保护订阅一直保留到发送协程停止。
					_ = process.Signal(stopSignal)
				}
			}
		}()
		defer func() { stopSender(); <-senderDone }()

		workerStopped := make(chan struct{})
		service := NewCores(
			WithInternalHTTPAddress("127.0.0.1:0"),
			WithWorker(func(ctx context.Context) error {
				<-ctx.Done()
				close(workerStopped)
				return ctx.Err()
			}),
		)
		if mode == "service" {
			err = Start(service)
		} else {
			err = Run(service)
		}
		if err != nil {
			t.Fatalf("manual shutdown returned an error: %v", err)
		}
		select {
		case <-workerStopped:
		default:
			t.Fatal("service returned before its Worker stopped")
		}
		if mode == "service" {
			assertAddrReleased(t, service.internalHTTP.listener.Addr().String())
		}
		return
	}

	for _, mode := range []string{"service", "script"} {
		for _, stopSignal := range []string{"interrupt", "terminate"} {
			t.Run(mode+"/"+stopSignal, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGracefulSignalsReturnSuccess$")
				cmd.Env = append(os.Environ(),
					"GOUTILS_GRACEFUL_TEST_MODE="+mode,
					"GOUTILS_GRACEFUL_TEST_SIGNAL="+stopSignal,
				)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("signal shutdown failed: %v\n%s", err, output)
				}
			})
		}
	}
}
