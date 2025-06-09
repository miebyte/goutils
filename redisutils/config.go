package redisutils

import (
	"fmt"

	"github.com/miebyte/goutils/discover"
	"github.com/pkg/errors"
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

type RedisPool map[string]*RedisClient

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

func (rc RedisConfigMap) DialGoRedisPool() (RedisPool, error) {
	rp := make(map[string]*RedisClient)

	for name := range rc {
		client, err := rc.DialGoRedisClient(name)
		if err != nil {
			return nil, errors.Wrapf(err, "dial redisPool of %s", name)
		}

		rp[name] = client
	}

	return rp, nil
}

func (rp RedisPool) GetRedis(names ...string) (*RedisClient, error) {
	name := "default"
	if len(names) != 0 {
		name = names[0]
	}

	c, exists := rp[name]
	if !exists {
		return nil, errors.Errorf("redis(%s) not init", name)
	}
	return c, nil
}

func (rp RedisPool) Close() {
	for _, client := range rp {
		_ = client.Close()
	}
}
