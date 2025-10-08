package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	sliceFlag := StringSlice("slice", []string{"default"}, "slice flag")
	assert.Equal(t, []string{"default"}, sliceFlag())

	sf.Set("slice", "test,test2")

	assert.Equal(t, []string{"test", "test2"}, sliceFlag())
}
