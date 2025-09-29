package utils

import (
	"fmt"
	"reflect"
	"testing"
)

// TestPairwiseWithEmptyOrSingleElement tests empty slice or single element slice
func TestPairwiseWithEmptyOrSingleElement(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected [][2]any
	}{
		{
			name:     "empty slice",
			input:    []any{},
			expected: [][2]any{},
		},
		{
			name:     "single element slice",
			input:    []any{1},
			expected: [][2]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([][2]any, 0)
			pairs := Pairwise(tt.input)
			for v1, v2 := range pairs {
				result = append(result, [2]any{v1, v2})
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Pairwise() got = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPairwiseWithInts tests integer slices
func TestPairwiseWithInts(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected [][2]int
	}{
		{
			name:     "two elements integer slice",
			input:    []int{1, 2},
			expected: [][2]int{{1, 2}},
		},
		{
			name:     "multiple elements integer slice (even)",
			input:    []int{1, 2, 3, 4},
			expected: [][2]int{{1, 2}, {3, 4}},
		},
		{
			name:     "multiple elements integer slice (odd)",
			input:    []int{1, 2, 3, 4, 5},
			expected: [][2]int{{1, 2}, {3, 4}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([][2]int, 0)
			pairs := Pairwise(tt.input)
			for v1, v2 := range pairs {
				result = append(result, [2]int{v1, v2})
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Pairwise() got = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPairwiseWithStrings tests string slices
func TestPairwiseWithStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected [][2]string
	}{
		{
			name:     "multiple elements string slice",
			input:    []string{"a", "b", "c"},
			expected: [][2]string{{"a", "b"}},
		},
		{
			name:     "two elements string slice",
			input:    []string{"x", "y"},
			expected: [][2]string{{"x", "y"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([][2]string, 0)
			pairs := Pairwise(tt.input)
			for v1, v2 := range pairs {
				result = append(result, [2]string{v1, v2})
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Pairwise() got = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPairwiseWithMixedTypes tests mixed type slices
func TestPairwiseWithMixedTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    []any
		expected [][2]any
	}{
		{
			name:     "mixed types with integers and strings",
			input:    []any{1, "b", 3.0},
			expected: [][2]any{{1, "b"}},
		},
		{
			name:     "mixed types with multiple types",
			input:    []any{1, true, "c", 2.5},
			expected: [][2]any{{1, true}, {"c", 2.5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([][2]any, 0)
			pairs := Pairwise(tt.input)
			for v1, v2 := range pairs {
				result = append(result, [2]any{v1, v2})
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Pairwise() got = %v, want %v", result, tt.expected)
			}
		})
	}
}

// user 用于测试 SliceToMap，且实现 MapKeyer 接口
type user struct {
	ID   int
	Name string
}

func (u user) GetKey() int {
	return u.ID
}

func TestSliceToMap(t *testing.T) {
	slice := []user{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
		{ID: 3, Name: "Carol"},
	}

	m := SliceToMap(slice)

	expected := map[int]user{
		1: {ID: 1, Name: "Alice"},
		2: {ID: 2, Name: "Bob"},
		3: {ID: 3, Name: "Carol"},
	}

	if !reflect.DeepEqual(m, expected) {
		t.Errorf("SliceToMap() got = %v, want %v", m, expected)
	}
}

// intStr 用于测试 SliceConvert，且实现 Converter[string] 接口
type intStr struct {
	Value int
}

func (i intStr) ConvertTo() string {
	return "num:" + fmt.Sprint(i.Value)
}

func TestSliceConvert(t *testing.T) {
	slice := []intStr{
		{Value: 1},
		{Value: 2},
		{Value: 3},
	}

	expected := []string{"num:1", "num:2", "num:3"}
	result := SliceConvert(slice)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("SliceConvert() got = %v, want %v", result, expected)
	}
}
