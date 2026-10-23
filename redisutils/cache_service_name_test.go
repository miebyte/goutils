package redisutils

import (
	"context"
	"strings"
	"testing"

	"github.com/miebyte/goutils/share"
)

func TestCacheKeyUsesSharedServiceName(t *testing.T) {
	original := share.GetServiceName()
	t.Cleanup(func() { share.SetServiceName(original) })
	share.SetServiceName("orders")
	cache := CacheCall(func(context.Context, int) (string, error) { return "value", nil }).SetPrefix("test")
	key, err := cache.buildRedisKey(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "test:orders:") {
		t.Fatalf("cache key = %q, want shared service name", key)
	}
}
