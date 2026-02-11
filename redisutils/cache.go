package redisutils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miebyte/goutils/internal/share"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/sync/singleflight"
)

type Serializer interface {
	Serialize(v any) ([]byte, error)
	Deserialize(data []byte, v any) error
	SN() string
}

type MsgPackSerializer struct{}

func (*MsgPackSerializer) Serialize(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (*MsgPackSerializer) Deserialize(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

func (*MsgPackSerializer) SN() string {
	return "msgpack"
}

type (
	AnyFn[Req, Ret any]  func(context.Context, Req) (Ret, error)
	ArgsKeyFunc[Req any] func(string, Req) (string, error)
)

// CacheContext 是缓存调用上下文。
// 用于设置缓存调用的各种参数，并最终调用 fn 获取结果。
// fn 调用结果会存储在指定的 redis 中，key 格式为"{prefix}:{serviceName}:{funcName}:{argsMd5}:{sn}"
// {serviceName} 为服务名称，用于区分不同的服务。通过 buildinfo.GetProjectName 获取
// {funcName} 为 fn 的名称
// {argsMd5} 为 fn 的参数的 md5 值
// {sn} 为 Serializer 的识别号，用于区分不同的序列化方式。
type CacheContext[Req, Ret any] struct {
	ctx     context.Context
	client  *RedisClient
	sfGroup singleflight.Group

	// prefix: 缓存key前缀。默认为 "redisutils:cache"
	prefix string

	// ttl: 缓存过期时间
	ttl time.Duration

	// ttlJitter：给 TTL 增加随机抖动，避免大量 key 同时过期（雪崩）
	ttlJitter time.Duration

	// argsKeyFunc 函数入参 key 自定义生成。返回 "" 的话会走默认策略。
	// 默认策略会使用 fn 的名称和 args 的 md5 值作为 key
	argsKeyFunc ArgsKeyFunc[Req]

	// serializer: 序列化器，用于将结果序列化后存储到 redis 中
	serializer Serializer

	// err 存放 CacheContext 内部的错误
	err error

	fn     AnyFn[Req, Ret]
	fnName string
}

type cacheResult[Ret any] struct {
	val Ret
	err error
}

func (cc *CacheContext[Req, Ret]) WithClient(client *RedisClient) *CacheContext[Req, Ret] {
	cc.client = client
	return cc
}

func (cc *CacheContext[Req, Ret]) SetPrefix(prefix string) *CacheContext[Req, Ret] {
	cc.prefix = prefix
	return cc
}

func (cc *CacheContext[Req, Ret]) SetTTL(ttl time.Duration) *CacheContext[Req, Ret] {
	cc.ttl = ttl
	return cc
}

func (cc *CacheContext[Req, Ret]) SetTTLJitter(ttlJitter time.Duration) *CacheContext[Req, Ret] {
	cc.ttlJitter = ttlJitter
	return cc
}

func (cc *CacheContext[Req, Ret]) SetArgsKeyFunc(argsKeyFunc ArgsKeyFunc[Req]) *CacheContext[Req, Ret] {
	cc.argsKeyFunc = argsKeyFunc
	return cc
}

func (cc *CacheContext[Req, Ret]) SetSerializer(serializer Serializer) *CacheContext[Req, Ret] {
	cc.serializer = serializer
	return cc
}

func (cc *CacheContext[Req, Ret]) ensureDefaults() {
	if cc.prefix == "" {
		cc.prefix = "redisutils:cache"
	}
	if cc.serializer == nil {
		cc.serializer = &MsgPackSerializer{}
	}
	if cc.fnName == "" && cc.fn != nil {
		cc.fnName = shortFuncName(logging.GetFuncName(cc.fn))
	}
	if cc.ctx == nil {
		cc.ctx = context.TODO()
	}
}

// cacheTTL: 计算缓存过期时间。
// 如果 ttlJitter 大于 0，则增加一个随机抖动。
func (cc *CacheContext[Req, Ret]) cacheTTL() time.Duration {
	if cc.ttlJitter <= 0 {
		return cc.ttl
	}
	maxNanos := cc.ttlJitter.Nanoseconds()
	if maxNanos <= 0 {
		return cc.ttl
	}

	jitter := utils.RandInt64(0, maxNanos)
	return cc.ttl + time.Duration(jitter)
}

// defaultFnArgsKey 默认 fn-args key 生成策略。
// 使用 fn 的名称和 args 的 md5 值作为 key
func (cc *CacheContext[Req, Ret]) defaultFnArgsKey(fnName string, req Req) string {
	buf := make([]byte, 0, 64)
	buf = append(buf, fnName...)
	buf = append(buf, '|')
	buf = utils.AppendAny(buf, req)
	return string(utils.Md5(buf))
}

// buildRedisKey 构建 redis key。
// 如果 keyFunc 不为 nil，则使用 keyFunc 生成 key。
// 最终 key 格式为 {prefix}:{serviceName}:{funcName}:{argsMd5}:{sn}
func (cc *CacheContext[Req, Ret]) buildRedisKey(ctx context.Context, req Req) (string, error) {
	cc.ensureDefaults()
	ctx = cc.getContext(ctx)

	keyPart := ""
	if cc.argsKeyFunc != nil {
		customKey, err := cc.argsKeyFunc(cc.fnName, req)
		if err != nil {
			return "", err
		}
		keyPart = customKey
	}
	if keyPart == "" {
		keyPart = cc.defaultFnArgsKey(cc.fnName, req)
	}

	serviceName := share.GetServiceName()
	if serviceName == "" {
		serviceName = "unknown"
	}

	return fmt.Sprintf("%s:%s:%s:%s:%s", cc.prefix, serviceName, cc.fnName, keyPart, cc.serializer.SN()), nil
}

func (cc *CacheContext[Req, Ret]) getContext(ctx context.Context) context.Context {
	if ctx == nil {
		return cc.ctx
	}
	return ctx
}

func (cc *CacheContext[Req, Ret]) shouldCache() bool {
	return cc.client != nil && cc.ttl > 0
}

func (cc *CacheContext[Req, Ret]) loadFromCache(ctx context.Context, key string) (Ret, bool) {
	var zero Ret
	if !cc.shouldCache() {
		return zero, false
	}
	data, err := cc.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false
	}
	if err != nil {
		logging.Errorc(ctx, "cache get failed: key=%s err=%v", key, err)
		return zero, false
	}

	var out Ret
	err = cc.serializer.Deserialize(data, &out)
	if err != nil {
		logging.Errorc(ctx, "cache value decode failed: key=%s err=%v", key, err)
		return zero, false
	}

	logging.Debugc(ctx, "loadFromCache: key=%s out=%v", key, out)
	return out, true
}

func (cc *CacheContext[Req, Ret]) saveToCache(ctx context.Context, key string, out Ret) error {
	if !cc.shouldCache() {
		return nil
	}
	data, err := cc.serializer.Serialize(out)
	if err != nil {
		return fmt.Errorf("cache encode failed: err=%w", err)
	}
	logging.Debugc(ctx, "saveToCache: ttl=%s key=%s", cc.cacheTTL(), key)
	err = cc.client.Set(ctx, key, data, cc.cacheTTL()).Err()
	if err != nil {
		return fmt.Errorf("cache save failed: err=%w", err)
	}
	return nil
}

func (cc *CacheContext[Req, Ret]) callFn(ctx context.Context, req Req) (Ret, error) {
	return cc.fn(ctx, req)
}

// makeCachedFunc 统一处理缓存调用逻辑。
func (cc *CacheContext[Req, Ret]) makeCachedFunc(ctx context.Context, req Req, once bool) (Ret, error) {
	var zero Ret
	if cc.err != nil {
		return zero, cc.err
	}

	cc.ensureDefaults()
	ctx = cc.getContext(ctx)

	if !cc.shouldCache() {
		logging.Warnf("%s not cache", cc.fnName)
		return cc.callFn(ctx, req)
	}

	key, err := cc.buildRedisKey(ctx, req)
	if err != nil {
		logging.Errorc(ctx, "buildRedisKey error: %v", err)
		return cc.callFn(ctx, req)
	}

	logging.Debugc(ctx, "buildRedisKey: key=%s", key)

	out, ok := cc.loadFromCache(ctx, key)
	if ok {
		return out, nil
	}

	if !once {
		return cc.callAndCache(ctx, key, req)
	}

	val, _, _ := cc.sfGroup.Do(key, func() (any, error) {
		out, err := cc.callAndCache(context.WithoutCancel(ctx), key, req)
		return cacheResult[Ret]{val: out, err: err}, nil
	})
	result, ok := val.(cacheResult[Ret])
	if !ok {
		return cc.callFn(ctx, req)
	}
	return result.val, result.err
}

func (cc *CacheContext[Req, Ret]) callAndCache(ctx context.Context, key string, req Req) (Ret, error) {
	out, err := cc.callFn(ctx, req)
	if err != nil {
		logging.Errorc(ctx, "callAndCache: has non nil error: err=%v", err)
		return out, err
	}
	err = cc.saveToCache(ctx, key, out)
	if err != nil {
		logging.Errorc(ctx, "callAndCache: save to cache failed: key=%s err=%v", key, err)
	}
	return out, nil
}

// Func 会返回一个新的函数
// 该函数和原函数签名一致，但会自动缓存结果
// 当构建缓存 Key, 缓存数据时发生错误，会返回原始函数调用结果
func (cc *CacheContext[Req, Ret]) Func() AnyFn[Req, Ret] {
	return func(ctx context.Context, req Req) (Ret, error) {
		return cc.makeCachedFunc(ctx, req, false)
	}
}

// OnceFunc 会返回一个新的函数
// 该函数和原函数签名一致，但会自动缓存结果
// 并且并发调用时只会执行一次，后续调用会返回缓存结果
func (cc *CacheContext[Req, Ret]) OnceFunc() AnyFn[Req, Ret] {
	return func(ctx context.Context, req Req) (Ret, error) {
		return cc.makeCachedFunc(ctx, req, true)
	}
}

// CacheCall 会返回一个 CacheContext 实例
// 该实例可以用于构建缓存函数:
// - Func() AnyFn[Req, Ret]
// - OnceFunc() AnyFn[Req, Ret]
//
// 支持的 fn 类型:
// func(ctx context.Context, req Req) (ret Ret, err error)
//
// 注意：req 和 ret 必须可序列化
// 支持的配置:
// - WithClient(client *RedisClient) *CacheContext[Req, Ret]
// - SetPrefix(prefix string) *CacheContext[Req, Ret]
// - SetTTL(ttl time.Duration) *CacheContext[Req, Ret]
// - SetTTLJitter(ttlJitter time.Duration) *CacheContext[Req, Ret]
// - SetArgsKeyFunc(argsKeyFunc ArgsKeyFunc[Req]) *CacheContext[Req, Ret]
// - SetSerializer(serializer Serializer) *CacheContext[Req, Ret]
// - Func() AnyFn[Req, Ret]
func CacheCall[Req, Ret any](ctx context.Context, fn AnyFn[Req, Ret]) *CacheContext[Req, Ret] {
	if ctx == nil {
		ctx = context.TODO()
	}

	cc := &CacheContext[Req, Ret]{
		ctx:        ctx,
		fn:         fn,
		ttl:        5 * time.Minute,
		ttlJitter:  1 * time.Minute,
		serializer: &MsgPackSerializer{},
	}

	if fn == nil {
		cc.err = fmt.Errorf("fn 不能为空")
	} else {
		cc.fnName = shortFuncName(logging.GetFuncName(fn))
	}

	cc.ctx = logging.With(cc.ctx, "fnName", cc.fnName)
	return cc
}

func shortFuncName(full string) string {
	if full == "" {
		return full
	}

	if i := strings.LastIndex(full, "."); i >= 0 && i+1 < len(full) {
		return full[i+1:]
	}

	return full
}
