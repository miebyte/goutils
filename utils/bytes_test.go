package utils

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestAppendAny_MapStable(t *testing.T) {
	m := map[string]int{
		"b": 2,
		"a": 1,
		"c": 3,
	}
	first := AppendAny(nil, m)
	for i := 0; i < 50; i++ {
		got := AppendAny(nil, m)
		if !bytes.Equal(first, got) {
			t.Fatalf("AppendAny map output not stable")
		}
	}
}

func TestAppendAny_SliceStable(t *testing.T) {
	s := []int{3, 1, 2}
	first := AppendAny(nil, s)
	for i := 0; i < 10; i++ {
		got := AppendAny(nil, s)
		if !bytes.Equal(first, got) {
			t.Fatalf("AppendAny slice output not stable")
		}
	}
}

type testStringer struct {
	val string
}

func (ts testStringer) String() string {
	return "str:" + ts.val
}

func TestAppendAny_AllTypes(t *testing.T) {
	now := time.Date(2026, 1, 30, 10, 20, 30, 0, time.UTC)
	var nilPtr *int
	type sample struct {
		A int
		B string
	}

	cases := []any{
		nil,
		[]byte("hello"),
		"world",
		"1",
		int(1),
		int8(2),
		int16(3),
		int32(4),
		int64(5),
		uint(6),
		uint8(7),
		uint16(8),
		uint32(9),
		uint64(10),
		float32(1.25),
		float64(2.5),
		true,
		now,
		time.Second * 3,
		errors.New("boom"),
		testStringer{val: "x"},
		sample{A: 10, B: "ok"},
		&sample{A: 20, B: "ptr"},
		nilPtr,
		[]int{3, 1, 2},
		[3]string{"a", "b", "c"},
		map[string]int{"b": 2, "a": 1, "c": 3},
		map[int]string{2: "b", 1: "a"},
		map[string]any{"k1": 1, "44": "v2"},
		map[string]map[int]string{"k1": {1: "v1", 2: "v2"}, "k2": {3: "v3", 4: "v4"}},
	}

	buf := make([]byte, 0, 1024)
	for _, v := range cases {
		buf = AppendAny(buf, v)
	}
	t.Log(string(buf))
}
