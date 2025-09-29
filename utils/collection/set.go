package collection

import (
	"iter"

	"github.com/miebyte/goutils/utils/lang"
)

// Set is a type-safe generic set collection.
// It's not thread-safe, use with synchronization for concurrent access.
type Set[T comparable] struct {
	data map[T]lang.PlaceholderType
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		data: make(map[T]lang.PlaceholderType),
	}
}

func (s *Set[T]) Add(items ...T) {
	for _, item := range items {
		s.data[item] = lang.Placeholder
	}
}

func (s *Set[T]) Clear() {
	clear(s.data)
}

func (s *Set[T]) Contains(item T) bool {
	_, ok := s.data[item]
	return ok
}

func (s *Set[T]) Count() int {
	return len(s.data)
}

func (s *Set[T]) Keys() []T {
	keys := make([]T, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	return keys
}

func (s *Set[T]) Remove(item T) {
	delete(s.data, item)
}

func (s *Set[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		for key := range s.data {
			if !yield(key) {
				break
			}
		}
	}
}
