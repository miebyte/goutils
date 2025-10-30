package redisutils

import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// TestRedisPool_GetRedisLazyLoad 验证懒加载模式仅在首次访问时拨号。
func TestRedisPool_GetRedisLazyLoad(t *testing.T) {
	confMap := RedisConfigMap{
		"default": {Server: "localhost:6379"},
	}

	rp, err := NewRedisPool(confMap, WithLazyLoad(true))
	assert.NoError(t, err)
	t.Cleanup(func() { rp.Close() })

	if _, exists := rp.pools.Load("default"); exists {
		t.Fatalf("lazy pool should not preload client")
	}

	client1, err := rp.GetRedis()
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	cached, exists := rp.pools.Load("default")
	assert.True(t, exists)
	assert.Equal(t, client1, cached)

	client2, err := rp.GetRedis()
	assert.NoError(t, err)
	assert.True(t, client1 == client2)
}

// TestRedisPool_Preload 确认非懒加载模式会在构造时预拨号。
func TestRedisPool_Preload(t *testing.T) {
	confMap := RedisConfigMap{
		"default": {Server: "localhost:6379"},
	}

	pool, err := confMap.DialGoRedisPool()
	assert.NoError(t, err)
	defer pool.Close()

	cached, exists := pool.pools.Load("default")
	assert.True(t, exists)
	assert.NotNil(t, cached)

	client, err := pool.GetRedis()
	assert.NoError(t, err)
	assert.True(t, client == cached)
}

// TestRedisPool_ConcurrentGetRedis 校验 singleflight 只创建单个实例。
func TestRedisPool_ConcurrentGetRedis(t *testing.T) {
	confMap := RedisConfigMap{
		"default": {Server: "localhost:6379"},
	}

	rp, err := NewRedisPool(confMap, WithLazyLoad(true))
	assert.NoError(t, err)
	t.Cleanup(func() { rp.Close() })

	type result struct {
		client *RedisClient
		err    error
	}

	const workers = 16
	resCh := make(chan result, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, err := rp.GetRedis()
			resCh <- result{client: client, err: err}
		}()
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	var shared *RedisClient
	count := 0
	for res := range resCh {
		assert.NoError(t, res.err)
		assert.NotNil(t, res.client)
		if shared == nil {
			shared = res.client
		} else {
			assert.True(t, shared == res.client)
		}
		count++
	}

	assert.Equal(t, workers, count)
	assert.Equal(t, 1, len(rp.pools.Keys()))
}

// TestRedisPool_Close 验证关闭后资源清理及错误状态。
func TestRedisPool_Close(t *testing.T) {
	confMap := RedisConfigMap{
		"default": {Server: "localhost:6379"},
	}

	rp, err := NewRedisPool(confMap, WithLazyLoad(true))
	assert.NoError(t, err)

	client, err := rp.GetRedis()
	assert.NoError(t, err)

	rp.Close()
	assert.True(t, rp.IsClosed())

	ctx := context.Background()
	pingErr := client.Ping(ctx).Err()
	assert.ErrorIs(t, pingErr, redis.ErrClosed)

	_, err = rp.GetRedis()
	assert.ErrorIs(t, err, errRedisPoolClosed)
}

// TestRedisPool_LazyLoadWithExcludeNamesPreload 验证懒加载 + excludeNames 仅预拨号指定名称
func TestRedisPool_LazyLoadWithExcludeNamesPreload(t *testing.T) {
	confMap := RedisConfigMap{
		"default": {Server: "localhost:6379"},
		"cache":   {Server: "localhost:6379"},
		"session": {Server: "localhost:6379"},
	}

	rp, err := NewRedisPool(confMap, WithLazyLoad(true, "default", "session"))
	assert.NoError(t, err)
	t.Cleanup(func() { rp.Close() })

	// 仅 default、session 在构造后已拨号
	_, ok := rp.pools.Load("default")
	assert.True(t, ok)
	_, ok = rp.pools.Load("session")
	assert.True(t, ok)
	_, ok = rp.pools.Load("cache")
	assert.False(t, ok)

	// 首次访问非 exclude 的名称时再拨号
	client, err := rp.GetRedis("cache")
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// 现在应当已包含三者
	keys := rp.pools.Keys()
	assert.ElementsMatch(t, []string{"default", "session", "cache"}, keys)
}
