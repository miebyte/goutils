package discover

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type fixedResolver string

func (r fixedResolver) Discover(context.Context, string) ([]string, error) {
	return []string{string(r)}, nil
}

func resetDefaultResolver(t *testing.T) {
	t.Helper()
	saved := defaultResolver.Load()
	t.Cleanup(func() { defaultResolver.Store(saved) })
	defaultResolver.Store(nil)
}

func TestDefaultResolverFallbackPreservesExplicitSelection(t *testing.T) {
	resetDefaultResolver(t)
	explicit := fixedResolver("local:8080")
	fallback := fixedResolver("consul:8080")
	SetDefaultResolver(explicit)
	require.False(t, SetDefaultResolverIfUnset(fallback))
	require.Equal(t, explicit, currentResolver())

	SetDefaultResolver(nil)
	require.True(t, SetDefaultResolverIfUnset(fallback), "clearing the resolver permits default initialization again")
	require.Equal(t, fallback, currentResolver())
	require.False(t, SetDefaultResolverIfUnset(fixedResolver("other:8080")))
	require.Equal(t, fallback, currentResolver())

	SetDefaultResolver(explicit)
	require.Equal(t, explicit, currentResolver(), "an explicit selection replaces an initialized default")
}

func TestConcurrentDefaultResolverInitialization(t *testing.T) {
	resetDefaultResolver(t)
	const contenders = 32
	var calls sync.WaitGroup
	var winners atomic.Int32
	start := make(chan struct{})
	for range contenders {
		calls.Go(func() {
			<-start
			if SetDefaultResolverIfUnset(fixedResolver("consul:8080")) {
				winners.Add(1)
			}
		})
	}
	close(start)
	calls.Wait()
	require.EqualValues(t, 1, winners.Load())
	require.Equal(t, fixedResolver("consul:8080"), currentResolver())

	// 显式设置与默认初始化同时发生时，两种执行顺序都应保留显式选择。
	SetDefaultResolver(nil)
	start = make(chan struct{})
	explicit := fixedResolver("local:8080")
	for range contenders {
		calls.Go(func() {
			<-start
			SetDefaultResolverIfUnset(fixedResolver("consul:8080"))
		})
	}
	calls.Go(func() {
		<-start
		SetDefaultResolver(explicit)
	})
	close(start)
	calls.Wait()
	require.Equal(t, explicit, currentResolver())
}
