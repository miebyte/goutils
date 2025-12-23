package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDedup(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "single element slice",
			input:    []int{1},
			expected: []int{1},
		},
		{
			name:     "multiple elements slice",
			input:    []int{1, 2, 3, 2, 1},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Dedup(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
