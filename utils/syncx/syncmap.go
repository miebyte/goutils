package syncx

import "sync"

type SyncMapX[K comparable, V any] struct {
	sync.Map
	zero V
}

func NewSyncMapX[K comparable, V any]() *SyncMapX[K, V] {
	return &SyncMapX[K, V]{}
}

// Load 获取指定 key 对应的值，若不存在返回零值与 false。
func (m *SyncMapX[K, V]) Load(key K) (V, bool) {
	v, ok := m.Map.Load(key)
	if !ok {
		return m.zero, false
	}
	vv, ok := v.(V)
	if !ok {
		return m.zero, false
	}
	return vv, true
}

// Store 设置指定 key 的值。
func (m *SyncMapX[K, V]) Store(key K, value V) {
	m.Map.Store(key, value)
}

// LoadOrStore 若 key 存在则返回已存在的值与 true，否则写入并返回新值与 false。
func (m *SyncMapX[K, V]) LoadOrStore(key K, value V) (V, bool) {
	actual, loaded := m.Map.LoadOrStore(key, value)
	if !loaded {
		return value, false
	}
	vv, ok := actual.(V)
	if !ok {
		return m.zero, false
	}
	return vv, true
}

// LoadAndDelete 获取并删除指定 key 的值。
func (m *SyncMapX[K, V]) LoadAndDelete(key K) (V, bool) {
	v, ok := m.Map.LoadAndDelete(key)
	if !ok {
		return m.zero, false
	}
	vv, ok := v.(V)
	if !ok {
		return m.zero, false
	}
	return vv, true
}

// Delete 删除指定 key。
func (m *SyncMapX[K, V]) Delete(key K) {
	m.Map.Delete(key)
}

// Swap 设置新值并返回旧值及其是否存在。
func (m *SyncMapX[K, V]) Swap(key K, value V) (V, bool) {
	old, loaded := m.Map.Swap(key, value)
	if !loaded {
		return m.zero, false
	}
	vv, ok := old.(V)
	if !ok {
		return m.zero, false
	}
	return vv, true
}

// CompareAndSwap 若当前值等于 old，则替换为 new 并返回 true。
func (m *SyncMapX[K, V]) CompareAndSwap(key K, old, new V) bool {
	return m.Map.CompareAndSwap(key, old, new)
}

// CompareAndDelete 若当前值等于 old，则删除并返回 true。
func (m *SyncMapX[K, V]) CompareAndDelete(key K, old V) bool {
	return m.Map.CompareAndDelete(key, old)
}

// Range 遍历所有键值对，f 返回 false 则停止。
func (m *SyncMapX[K, V]) Range(f func(key K, value V) bool) {
	m.Map.Range(func(k, v any) bool {
		kk, ok1 := k.(K)
		vv, ok2 := v.(V)
		if !ok1 || !ok2 {
			return true
		}
		return f(kk, vv)
	})
}

// Has 判断 key 是否存在。
func (m *SyncMapX[K, V]) Has(key K) bool {
	_, ok := m.Map.Load(key)
	return ok
}

// Keys 返回当前所有键的快照。
func (m *SyncMapX[K, V]) Keys() []K {
	keys := make([]K, 0)
	m.Range(func(k K, _ V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

// Values 返回当前所有值的快照。
func (m *SyncMapX[K, V]) Values() []V {
	vals := make([]V, 0)
	m.Range(func(_ K, v V) bool {
		vals = append(vals, v)
		return true
	})
	return vals
}
