package redisutils

import (
	"slices"
	"sync/atomic"

	"github.com/miebyte/goutils/utils/syncx"
	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

var errRedisPoolClosed = errors.New("redis pool closed")

type RedisPool struct {
	lazyLoad bool
	// 懒加载模式下，排除的名称(即仍然会进行预连接)
	excludeNames []string
	closed       *atomic.Bool
	configs      RedisConfigMap
	pools        *syncx.SyncMapX[string, *RedisClient]
	dialGroup    *singleflight.Group
}

type redisPoolOptionsFunc func(*RedisPool)

func WithLazyLoad(lazyLoad bool, excludeNames ...string) redisPoolOptionsFunc {
	return func(rp *RedisPool) {
		rp.lazyLoad = lazyLoad
		rp.excludeNames = excludeNames
	}
}

func NewRedisPool(configs RedisConfigMap, opts ...redisPoolOptionsFunc) (*RedisPool, error) {
	rp := &RedisPool{
		lazyLoad:  false,
		configs:   configs,
		closed:    new(atomic.Bool),
		dialGroup: &singleflight.Group{},
		pools:     syncx.NewSyncMapX[string, *RedisClient](),
	}

	for _, opt := range opts {
		opt(rp)
	}

	initNames := make([]string, 0, len(configs))
	for name := range configs {
		if rp.lazyLoad && slices.Contains(rp.excludeNames, name) {
			initNames = append(initNames, name)
		} else if !rp.lazyLoad {
			initNames = append(initNames, name)
		}
	}

	for _, name := range initNames {
		_, err := rp.GetRedis(name)
		if err != nil {
			return nil, err
		}
	}

	return rp, nil
}

func (rp *RedisPool) GetRedis(names ...string) (*RedisClient, error) {
	if rp.IsClosed() {
		return nil, errRedisPoolClosed
	}

	name := "default"
	if len(names) != 0 {
		name = names[0]
	}

	// fast path
	if client, exists := rp.pools.Load(name); exists {
		if rp.IsClosed() {
			return nil, errRedisPoolClosed
		}
		return client, nil
	}

	if _, exists := rp.configs[name]; !exists {
		return nil, errors.Errorf("redis(%s) config not exists", name)
	}

	clientAny, err, _ := rp.dialGroup.Do(name, func() (any, error) {
		return rp.dialRedis(name)
	})
	if err != nil {
		return nil, err
	}

	client, ok := clientAny.(*RedisClient)
	if !ok || client == nil {
		return nil, errors.Errorf("redis(%s) client type mismatch", name)
	}

	if rp.IsClosed() {
		return nil, errRedisPoolClosed
	}

	return client, nil
}

func (rp *RedisPool) dialRedis(name string) (*RedisClient, error) {
	if rp.IsClosed() {
		return nil, errRedisPoolClosed
	}

	dialed, err := rp.configs.DialGoRedisClient(name)
	if err != nil {
		return nil, errors.Wrapf(err, "dial redisPool of %s", name)
	}

	if rp.IsClosed() {
		_ = dialed.Close()
		return nil, errRedisPoolClosed
	}

	rp.pools.Store(name, dialed)
	return dialed, nil
}

func (rp *RedisPool) Close() {
	rp.closed.Store(true)
	rp.pools.Range(func(key string, client *RedisClient) bool {
		_ = client.Close()
		rp.pools.Delete(key)
		return true
	})
}

func (rp *RedisPool) IsClosed() bool {
	return rp.closed.Load()
}
