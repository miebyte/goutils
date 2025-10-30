package redisutils

import (
	"fmt"

	"github.com/miebyte/goutils/discover"
)

type RedisConfig struct {
	Server   string `json:"server"`
	Db       int    `json:"db"`
	Username string `json:"username"`
	Password string `json:"password"`
	MinSize  int    `json:"minsize"`
	MaxSize  int    `json:"maxsize"`
	PoolSize int    `json:"poolsize"`
	Timeout  int    `json:"timeout"`
}

func (rc *RedisConfig) Address() string {
	return discover.GetServiceFinder().GetAddress(rc.Server)
}

func (conf *RedisConfig) DialGORedisClient() (*RedisClient, error) {
	return NewRedisClient(conf)
}

func (conf *RedisConfig) SetDefault() {
	if conf.Server == "" {
		conf.Server = "localhost:6379"
	}

	if conf.Db == 0 {
		conf.Db = 0
	}

	if conf.PoolSize == 0 {
		conf.PoolSize = 200
	}

	if conf.MinSize == 0 {
		conf.MinSize = 10
	}

	if conf.MaxSize == 0 {
		conf.MaxSize = 50
	}
}

type RedisConfigMap map[string]*RedisConfig

func (rc RedisConfigMap) DialGoRedisClient(names ...string) (*RedisClient, error) {
	name := "default"
	if len(names) != 0 {
		name = names[0]
	}

	conf, exists := rc[name]
	if !exists {
		return nil, fmt.Errorf("redis(%s) config not exists", name)
	}

	return conf.DialGORedisClient()
}

// Deprecated: 当前方法已废弃并将在后续版本移除, 请使用 NewRedisPool 替代
func (rc RedisConfigMap) DialGoRedisPool(opts ...redisPoolOptionsFunc) (RedisPool, error) {
	rp, err := NewRedisPool(rc, opts...)
	if err != nil {
		return RedisPool{}, err
	}

	return *rp, nil
}

func (rc RedisConfigMap) NewRedisPool(opts ...redisPoolOptionsFunc) (*RedisPool, error) {
	return NewRedisPool(rc, opts...)
}
