package utils

import (
	"maps"
	"slices"
)

// MapKeys 返回map的所有key组成的切片。
func MapKeys[Map ~map[K]V, K comparable, V any](m Map) []K {
	return slices.Collect(maps.Keys(m))
}

// MapValues 返回map的所有value组成的切片。
func MapValues[Map ~map[K]V, K comparable, V any](m Map) []V {
	return slices.Collect(maps.Values(m))
}
