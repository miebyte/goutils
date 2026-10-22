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

type structOption struct {
	key string

	isFullPath bool
	path       string

	provider      ConfigProvider
	configRefresh any // 保存 ConfigRefreshFunc[T]，由 Struct[T] 校验并提取。
}

type structOptionFunc func(*structOption)

type ConfigRefreshFunc[T any] func(T) error

func (f structOptionFunc) applyStruct(o *structOption) { f(o) }

// WithConfigPath 指定当前 key 需要读取的配置后缀。
// 对于 Consul:
//
// path 无需包含节点前缀，例如 "/configs/{serviceName}/{xxx}" 只需传入 "{xxx}"。
func WithConfigPath(path string) structOptionFunc {
	return func(o *structOption) {
		o.path = path
		o.isFullPath = false
	}
}

// WithFullConfigPath 指定完整配置路径，跳过根前缀和当前服务名。
func WithFullConfigPath(path string) structOptionFunc {
	return func(o *structOption) {
		o.path = path
		o.isFullPath = true
	}
}

// WithConfigRefresh 设置热更新回调，传入解析、补默认值并校验后的只读快照。
// 回调在新快照发布后执行，首次 Parser 初始化不调用；未设置回调时仅更新快照。
// 回调返回的错误由监听器记录，不会回滚已发布的快照。
func WithConfigRefresh[T any](fn ConfigRefreshFunc[T]) structOptionFunc {
	return func(o *structOption) { o.configRefresh = fn }
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
func Struct[T any](key string, defaultVal T, usage string, opts ...structOptionFunc) StructParser[T] {
	st := reflect.TypeFor[T]()
	if st.Kind() != reflect.Pointer || (st.Elem().Kind() != reflect.Struct && st.Elem().Kind() != reflect.Map) {
		logging.PanicError(fmt.Errorf("flags.Struct[%s] must be a pointer to a struct or map", key))
	}
	if strings.TrimSpace(key) == "" {
		logging.PanicError(fmt.Errorf("flags.Struct key must not be empty"))
	}

	var (
		mu  sync.Mutex
		opt structOption
	)

	for _, option := range opts {
		option.applyStruct(&opt)
	}
	opt.key = key

	configRefresh, ok := opt.configRefresh.(ConfigRefreshFunc[T])
	if opt.configRefresh != nil && !ok {
		logging.PanicError(fmt.Errorf("Struct %q refresh callback must be %T, got %T", key, configRefresh, opt.configRefresh))
	}
	owner := sf

	initialized := false
	value := new(AtomicValue[T])

	owner.SetDefault(key, defaultVal)
	owner.BindStructKey(opt, func(raw json.RawMessage) error {
		mu.Lock()
		defer mu.Unlock()

		next, err := decodeStruct[T](key, raw)
		if err != nil {
			return err
		}
		value.store(next)

		if configRefresh != nil {
			return configRefresh(next)
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
