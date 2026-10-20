package flags

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/miebyte/goutils/logging"
)

type structOption[T any] struct {
	path          string
	provider      ConfigProvider
	configRefresh ConfigRefreshFunc[T]
}

type (
	structOptionFunc[T any] func(*structOption[T])

	ConfigRefreshFunc[T any] func(T) error
)

func (f structOptionFunc[T]) applyStruct(o *structOption[T]) { f(o) }

// WithConfigPath 指定完整 ZK 路径或完整 Nacos Data ID。
func WithConfigPath[T any](path string) structOptionFunc[T] {
	return func(o *structOption[T]) { o.path = path }
}

// WithConfigRefresh 设置热更新回调，传入解析、补默认值并校验后的只读快照。
// 回调在新快照发布后执行，首次 Parser 初始化不调用；未设置回调时仅更新快照。
// 回调返回的错误由监听器记录，不会回滚已发布的快照。
func WithConfigRefresh[T any](fn ConfigRefreshFunc[T]) structOptionFunc[T] {
	return func(o *structOption[T]) { o.configRefresh = fn }
}

type HasDefault interface {
	SetDefault()
}

type HasValidator interface {
	Validate() error
}

// StructParser 应在 Parse 后调用，每个 Parser 只能成功初始化一次。
type StructParser[T any] func() (*AtomicValue[T], error)

// Struct 声明结构化 JSON 配置，T 必须是指向 struct 或 map 的指针。
// 声明阶段只登记 key 和选项；Parse 读取配置后，返回的 Parser 只解析缓存并发布初值。
// defaultVal 仅在首次读取缺少配置时使用，按 JSON 语义复制，不与已有配置合并。
// 每次解析创建新对象，执行 SetDefault、Validate，成功后才发布。
func Struct[T any](key string, defaultVal T, usage string, opts ...structOptionFunc[T]) StructParser[T] {
	st := reflect.TypeFor[T]()
	if st.Kind() != reflect.Pointer || (st.Elem().Kind() != reflect.Struct && st.Elem().Kind() != reflect.Map) {
		logging.PanicError(fmt.Errorf("superflags.Struct[%s] must be a pointer to a struct or map", key))
	}
	if strings.TrimSpace(key) == "" {
		logging.PanicError(fmt.Errorf("superflags.Struct key must not be empty"))
	}

	var (
		mu  sync.Mutex
		opt structOption[T]
	)

	for _, option := range opts {
		option.applyStruct(&opt)
	}
	owner := sf

	initialized := false
	value := new(AtomicValue[T])

	owner.SetDefault(key, defaultVal)
	owner.bindStructKey(key, opt.path, opt.provider, func(raw json.RawMessage) error {
		mu.Lock()
		defer mu.Unlock()

		next, err := decodeStruct[T](key, raw)
		if err != nil {
			return err
		}
		value.store(next)

		if opt.configRefresh != nil {
			return opt.configRefresh(next)
		}

		return nil
	})

	return func() (*AtomicValue[T], error) {
		mu.Lock()
		defer mu.Unlock()
		if initialized {
			return nil, fmt.Errorf("Struct %q has already been initialized", key)
		}

		// 监听可能先于 Parser 收到更新，已有快照时直接使用，避免旧缓存覆盖它。
		if value.current.Load() == nil {
			next, err := decodeStruct[T](key, owner.get(key))
			if err != nil {
				return nil, fmt.Errorf("parse Struct %q: %w", key, err)
			}
			value.store(next)
		}
		initialized = true
		return value, nil
	}
}

// decodeStruct 将一个 value 解析为目标类型 T。
func decodeStruct[T any](key string, value any) (T, error) {
	st := reflect.TypeFor[T]()
	next := reflect.New(st.Elem()).Convert(st).Interface().(T)
	var raw json.RawMessage
	switch v := value.(type) {
	case json.RawMessage:
		raw = v
	case string:
		raw = json.RawMessage(v)
	default:
		// 缺少配置时 get 会返回默认对象，复制到 next 后再执行校验。
		if v, ok := value.(T); ok && reflect.ValueOf(v).IsNil() {
			return next, nil
		}

		var err error
		raw, err = json.Marshal(value)
		if err != nil {
			return next, fmt.Errorf("key %q marshal failed: %w", key, err)
		}
		if bytes.Equal(raw, []byte("null")) {
			raw = nil
		}
	}

	if raw != nil {
		if !json.Valid(raw) {
			return next, fmt.Errorf("key %q is not a valid JSON object", key)
		}

		if err := json.Unmarshal(raw, next); err != nil {
			return next, fmt.Errorf("key %q unmarshal failed: %w", key, err)
		}
	}

	err := checkStruct(next)
	if err != nil {
		return next, fmt.Errorf("key %q check failed: %w", key, err)
	}

	return next, nil
}

func checkStruct(next any) error {
	if d, ok := any(next).(HasDefault); ok {
		d.SetDefault()
	}
	if v, ok := any(next).(HasValidator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}
