package utils

import (
	"reflect"
	"testing"
)

// TestMap tests the Map function
func TestMap(t *testing.T) {
	input := []int{1, 2, 3}
	expected := []int{2, 4, 6}
	result := Map(input, func(x int) int { return x * 2 })
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestReduce tests the Reduce function
func TestReduce(t *testing.T) {
	input := []int{1, 2, 3}
	expected := 6
	result := Reduce(input, 0, func(acc, v int) int { return acc + v })
	if result != expected {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestFilter tests the Filter function
func TestFilter(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expected := []int{2, 4}
	result := Filter(input, func(x int) bool { return x%2 == 0 })
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestAny tests the Any function
func TestAny(t *testing.T) {
	input := []int{1, 2, 3}
	result := Any(input, func(x int) bool { return x%2 == 0 })
	if !result {
		t.Error("Expected true, got false")
	}
}

// TestAll tests the All function
func TestAll(t *testing.T) {
	input := []int{2, 4, 6}
	result := All(input, func(x int) bool { return x%2 == 0 })
	if !result {
		t.Error("Expected true, got false")
	}
}

// TestFind tests the Find function
func TestFind(t *testing.T) {
	input := []int{1, 2, 3}
	expected := 2
	result, found := Find(input, func(x int) bool { return x%2 == 0 })
	if !found || result != expected {
		t.Errorf("Expected %v, found %v, got %v", expected, found, result)
	}
}

// TestGroupBy tests the GroupBy function
func TestGroupBy(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expected := map[bool][]int{true: {2, 4}, false: {1, 3}}
	result := GroupBy(input, func(x int) bool { return x%2 == 0 })
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestZip tests the Zip function
func TestZip(t *testing.T) {
	input1 := []int{1, 2}
	input2 := []string{"a", "b"}
	expected := [][2]any{{1, "a"}, {2, "b"}}
	result := Zip(input1, input2)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestPartition tests the Partition function
func TestPartition(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expectedTrue := []int{2, 4}
	expectedFalse := []int{1, 3}
	trueSlice, falseSlice := Partition(input, func(x int) bool { return x%2 == 0 })
	if !reflect.DeepEqual(trueSlice, expectedTrue) || !reflect.DeepEqual(falseSlice, expectedFalse) {
		t.Errorf("Expected %v and %v, got %v and %v", expectedTrue, expectedFalse, trueSlice, falseSlice)
	}
}

// TestContains tests the Contains function
func TestContains(t *testing.T) {
	input := []int{1, 2, 3}
	result := Contains(input, 2)
	if !result {
		t.Error("Expected true, got false")
	}
}

// TestMapIter tests the MapIter function
func TestMapIter(t *testing.T) {
	input := []int{1, 2, 3}
	expected := []int{2, 4, 6}
	var result []int
	for v := range MapIter(input, func(x int) int { return x * 2 }) {
		result = append(result, v)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestFilterIter tests the FilterIter function
func TestFilterIter(t *testing.T) {
	input := []int{1, 2, 3, 4}
	expected := []int{2, 4}
	var result []int
	for v := range FilterIter(input, func(x int) bool { return x%2 == 0 }) {
		result = append(result, v)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

// TestZipIter tests the ZipIter function
func TestZipIter(t *testing.T) {
	input1 := []int{1, 2}
	input2 := []string{"a", "b"}
	expected := [][2]any{{1, "a"}, {2, "b"}}
	var result [][2]any
	for v := range ZipIter(input1, input2) {
		result = append(result, v)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}
