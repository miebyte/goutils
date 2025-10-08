package collection

import (
	"iter"
	"maps"
	"slices"
)

type DefaultDict[K comparable, V any] struct {
	m       map[K]V
	factory func() V
}

func NewDefaultDict[K comparable, V any](factory func() V) *DefaultDict[K, V] {
	return &DefaultDict[K, V]{
		m:       make(map[K]V),
		factory: factory,
	}
}

func (d *DefaultDict[K, V]) Get(key K) V {
	v, ok := d.m[key]
	if !ok {
		v = d.factory()
		d.m[key] = v
	}
	return v
}

func (d *DefaultDict[K, V]) Set(key K, value V) {
	d.m[key] = value
}

func (d *DefaultDict[K, V]) Map() map[K]V {
	return d.m
}

func (d *DefaultDict[K, V]) Items() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, value := range d.m {
			if !yield(key, value) {
				return
			}
		}
	}
}

func (d *DefaultDict[K, V]) Keys() []K {
	return slices.Collect(maps.Keys(d.m))
}

func (d *DefaultDict[K, V]) Values() []V {
	return slices.Collect(maps.Values(d.m))
}

type DefaultDictSlice[K comparable, E any, V []E] struct {
	m       map[K]V
	factory func() V
}

func NewDefaultDictSlice[K comparable, E any, V []E]() *DefaultDictSlice[K, E, V] {
	return &DefaultDictSlice[K, E, V]{
		m:       make(map[K]V),
		factory: func() V { return make(V, 0) },
	}
}

func (d *DefaultDictSlice[K, E, V]) Get(key K) V {
	v, ok := d.m[key]
	if !ok {
		v = d.factory()
		d.m[key] = v
	}
	return v
}

func (d *DefaultDictSlice[K, E, V]) Set(key K, value V) {
	d.m[key] = value
}

func (d *DefaultDictSlice[K, E, V]) Append(key K, values ...E) {
	v, ok := d.m[key]
	if !ok {
		v = d.factory()
	}
	v = append(v, values...)
	d.m[key] = v
}

func (d *DefaultDictSlice[K, E, V]) Map() map[K]V {
	return d.m
}

func (d *DefaultDictSlice[K, E, V]) Items() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for key, value := range d.m {
			if !yield(key, value) {
				return
			}
		}
	}
}

func (d *DefaultDictSlice[K, E, V]) Keys() []K {
	return slices.Collect(maps.Keys(d.m))
}

func (d *DefaultDictSlice[K, E, V]) Values() []V {
	return slices.Collect(maps.Values(d.m))
}
