package utils

import (
	"testing"
)

func TestMapKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := MapKeys(m)
	if len(keys) != 3 {
		t.Errorf("期望key数量为3，实际为%d", len(keys))
	}
	expect := map[string]bool{"a": true, "b": true, "c": true}
	for _, k := range keys {
		if !expect[k] {
			t.Errorf("未期望的key: %v", k)
		}
	}
}

func TestMapValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	values := MapValues(m)
	if len(values) != 3 {
		t.Errorf("期望value数量为3，实际为%d", len(values))
	}
	expect := map[int]bool{1: true, 2: true, 3: true}
	for _, v := range values {
		if !expect[v] {
			t.Errorf("未期望的value: %v", v)
		}
	}
}
