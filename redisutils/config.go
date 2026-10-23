package redisutils

import (
	"fmt"
	"time"
)

type RedisConfig struct {
	Server   string `json:"server"`
	Db       int    `json:"db"`
	Username string `json:"username"`
	Password string `json:"password"`
	MinSize  int    `json:"minsize"`
	MaxSize  int    `json:"maxsize"`
	PoolSize int    `json:"poolsize"`
	// Timeout 是连接、读取和写入的超时秒数，为 0 时默认 5 秒。
	Timeout int `json:"timeout"`
	// ContextTimeoutEnabled 让命令读写遵循调用方 Context 的超时和截止时间。
	ContextTimeoutEnabled bool `json:"context_timeout_enabled"`
	// MaxRetries 为 0 时默认重试 3 次，为 -1 时禁用重试。
	MaxRetries int `json:"max_retries"`
}

// Validate 校验配置，不修改调用方传入的值。
func (conf *RedisConfig) Validate() error {
	if conf == nil {
		return fmt.Errorf("redis config must not be nil")
	}
	if conf.Db < 0 {
		return fmt.Errorf("redis db must not be negative")
	}
	if conf.Timeout < 0 || int64(conf.Timeout) > int64((1<<63-1)/time.Second) {
		return fmt.Errorf("redis timeout must be a non-negative number of seconds within time.Duration range")
	}
	if conf.MaxRetries < -1 {
		return fmt.Errorf("redis max_retries must be -1 or greater")
	}
	return nil
}

// Address 返回 Redis 连接地址。
func (rc *RedisConfig) Address() string {
	return rc.Server
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

	if conf.Timeout == 0 {
		conf.Timeout = 5
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
