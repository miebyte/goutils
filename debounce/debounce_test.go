package debounce

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLessExecutor_Basic 验证基础节流行为：首次执行、窗口内丢弃、窗口外再次执行。
func TestLessExecutor_Basic(t *testing.T) {
	le := NewLessExecutor(50 * time.Millisecond)

	var execCount int32

	if ok := le.DoOrDiscard(func() {
		atomic.AddInt32(&execCount, 1)
	}); !ok {
		t.Fatalf("first call should execute")
	}

	if ok := le.DoOrDiscard(func() {
		atomic.AddInt32(&execCount, 1)
	}); ok {
		t.Fatalf("second call within threshold should be discarded")
	}

	if got := atomic.LoadInt32(&execCount); got != 1 {
		t.Fatalf("execCount=%d, want=1 after second call", got)
	}

	time.Sleep(60 * time.Millisecond)

	if ok := le.DoOrDiscard(func() {
		atomic.AddInt32(&execCount, 1)
	}); !ok {
		t.Fatalf("call after threshold should execute")
	}

	if got := atomic.LoadInt32(&execCount); got != 2 {
		t.Fatalf("execCount=%d, want=2 after third call", got)
	}
}

// TestLessExecutor_Concurrent 验证在高并发场景下，窗口内仅有一次执行。
func TestLessExecutor_Concurrent(t *testing.T) {
	le := NewLessExecutor(100 * time.Millisecond)

	var execCount int32
	const goroutines = 128

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			le.DoOrDiscard(func() {
				atomic.AddInt32(&execCount, 1)
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&execCount); got != 1 {
		t.Fatalf("execCount=%d, want=1 after first burst", got)
	}

	time.Sleep(120 * time.Millisecond)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			le.DoOrDiscard(func() {
				atomic.AddInt32(&execCount, 1)
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&execCount); got != 2 {
		t.Fatalf("execCount=%d, want=2 after second burst", got)
	}
}
