package utils

import (
	"fmt"
	"iter"
)

// Pairwise returns a sequence of pairs of adjacent elements from the slice.
// For a slice [a, b, c, d], it yields pairs (a, b), (c, d).
// If the slice has fewer than 2 elements, it returns an empty sequence.
func Pairwise[T any](s []T) iter.Seq2[T, T] {
	return func(yield func(T, T) bool) {
		for i := 0; i+1 < len(s); i += 2 {
			if !yield(s[i], s[i+1]) {
				return
			}
		}
	}
}

func Convert[S, T any](s []S, fn func(S) T) []T {
	return []T(AsSlice(s).Map(fn))
}

type Converter[T any] interface {
	ConvertTo() T
}

func SliceConvert[T any, S Converter[T]](s []S) []T {
	result := make([]T, len(s))
	for i, v := range s {
		result[i] = v.ConvertTo()
	}
	return result
}

type MapKeyer[K comparable] interface {
	GetKey() K
}

func SliceToMap[K comparable, E MapKeyer[K]](s []E) map[K]E {
	m := make(map[K]E, len(s))
	for _, v := range s {
		m[v.GetKey()] = v
	}

	return m
}

func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Slice adds fluent collection operations to a slice.
// Converting a slice with AsSlice does not copy its backing array.
type Slice[E any] []E

// Pair holds two values that may have different types.
type Pair[A, B any] struct {
	First  A
	Second B
}

func (p Pair[A, B]) String() string {
	return fmt.Sprintf("[%v, %v]", p.First, p.Second)
}

// AsSlice converts any slice type to Slice without copying its backing array.
func AsSlice[S ~[]E, E any](s S) Slice[E] {
	return Slice[E](s)
}

// Map applies fn to every element and returns the transformed values.
func (s Slice[E]) Map[R any](fn func(E) R) Slice[R] {
	result := make(Slice[R], len(s))
	for i, value := range s {
		result[i] = fn(value)
	}
	return result
}

// Reduce folds the slice into a value, starting with initial.
func (s Slice[E]) Reduce[R any](initial R, fn func(R, E) R) R {
	result := initial
	for _, value := range s {
		result = fn(result, value)
	}
	return result
}

// GroupBy groups elements using the key returned by keyFn.
func (s Slice[E]) GroupBy[K comparable](keyFn func(E) K) map[K]Slice[E] {
	result := make(map[K]Slice[E])
	for _, value := range s {
		key := keyFn(value)
		result[key] = append(result[key], value)
	}
	return result
}

// Zip pairs elements at the same index and stops at the shorter slice.
func (s Slice[E]) Zip[U any](other []U) []Pair[E, U] {
	length := min(len(s), len(other))
	result := make([]Pair[E, U], length)
	for i := range length {
		result[i] = Pair[E, U]{First: s[i], Second: other[i]}
	}
	return result
}

// MapIter lazily applies fn to every element.
func (s Slice[E]) MapIter[R any](fn func(E) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for _, value := range s {
			if !yield(fn(value)) {
				return
			}
		}
	}
}
