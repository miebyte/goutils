package superflags

import "sync/atomic"

// AtomicValue 保存 Struct 发布的不可变配置快照。第一次使用后不得复制。
// Load 返回的对象（包括 map、slice 和指针字段）必须只读。
type AtomicValue[T any] struct {
	current atomic.Pointer[T]
}

func (v *AtomicValue[T]) Load() T {
	current := v.current.Load()
	if current == nil {
		var zero T
		return zero
	}
	return *current
}

func (v *AtomicValue[T]) store(next T) {
	v.current.Store(&next)
}
