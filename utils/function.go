// File:		function.go
// Created by:	Hoven
// Created on:	2025-04-25
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package utils

import "iter"

// Map applies a function to each element in a slice and returns a new slice
func Map[T any, U any](arr []T, fn func(T) U) []U {
	result := make([]U, len(arr))
	for i, v := range arr {
		result[i] = fn(v)
	}
	return result
}

// Reduce reduces a slice to a single value using a provided function and initial value
func Reduce[T any, U any](arr []T, initial U, fn func(acc U, v T) U) U {
	result := initial
	for _, v := range arr {
		result = fn(result, v)
	}
	return result
}

// Filter returns a new slice containing elements that satisfy a predicate function
func Filter[T any](arr []T, predicate func(T) bool) []T {
	result := []T{}
	for _, v := range arr {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Any checks if any element in the slice satisfies the predicate function
func Any[T any](arr []T, predicate func(T) bool) bool {
	for _, v := range arr {
		if predicate(v) {
			return true
		}
	}
	return false
}

// All checks if all elements in the slice satisfy the predicate function
func All[T any](arr []T, predicate func(T) bool) bool {
	for _, v := range arr {
		if !predicate(v) {
			return false
		}
	}
	return true
}

// Find returns the first element in the slice that satisfies the predicate function
func Find[T any](arr []T, predicate func(T) bool) (T, bool) {
	for _, v := range arr {
		if predicate(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// GroupBy groups elements in the slice by a specified key function
func GroupBy[T any, K comparable](arr []T, keyFunc func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, v := range arr {
		key := keyFunc(v)
		result[key] = append(result[key], v)
	}
	return result
}

// Zip combines two slices into a slice of pairs (tuples)
func Zip[T1 any, T2 any](arr1 []T1, arr2 []T2) [][2]any {
	length := len(arr1)
	if len(arr2) < length {
		length = len(arr2)
	}
	result := make([][2]any, length)
	for i := 0; i < length; i++ {
		result[i] = [2]any{arr1[i], arr2[i]}
	}
	return result
}

// Partition divides a slice into two slices based on a predicate function
func Partition[T any](arr []T, predicate func(T) bool) ([]T, []T) {
	var trueSlice, falseSlice []T
	for _, v := range arr {
		if predicate(v) {
			trueSlice = append(trueSlice, v)
		} else {
			falseSlice = append(falseSlice, v)
		}
	}
	return trueSlice, falseSlice
}

func Contains[T comparable](arr []T, tar T) bool {
	for _, a := range arr {
		if a == tar {
			return true
		}
	}
	return false
}

func MapIter[inputs ~[]E, E any, U any](arr inputs, fn func(E) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for _, item := range arr {
			if mapped := fn(item); !yield(mapped) {
				return
			}
		}
	}
}

func FilterIter[inputs ~[]E, E any](arr inputs, fn func(E) bool) iter.Seq[E] {
	return func(yield func(E) bool) {
		for _, item := range arr {
			if !fn(item) {
				continue
			}

			if !yield(item) {
				return
			}
		}
	}
}

func ZipIter[T1 any, T2 any, E [2]any](arr1 []T1, arrs []T2) iter.Seq[E] {
	return func(yield func(E) bool) {
		length := len(arr1)
		if len(arrs) < length {
			length = len(arrs)
		}

		for i := 0; i < length; i++ {
			zipped := [2]any{arr1[i], arrs[i]}
			if !yield(zipped) {
				return
			}
		}
	}
}
