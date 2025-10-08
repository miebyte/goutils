package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	stringFlag := String("string", "default", "string flag")
	stringRequiredFlag := StringRequired("string_required", "string required flag")

	sf.Set("string", "test")
	sf.Set("string_required", "test")

	assert.Equal(t, "test", stringFlag())
	assert.Equal(t, "test", stringRequiredFlag())
}
