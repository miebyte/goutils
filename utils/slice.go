// File:		slice.go
// Created by:	Hoven
// Created on:	2025-04-04
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package utils

import "iter"

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
	result := make([]T, len(s))
	for i, v := range s {
		result[i] = fn(v)
	}
	return result
}
