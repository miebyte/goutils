package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBool(t *testing.T) {
	boolFlag := Bool("bool", false, "bool flag")
	boolRequiredFlag := BoolRequired("bool_required", "bool required flag")

	sf.Set("bool", "true")
	sf.Set("bool_required", "true")

	assert.Equal(t, true, boolFlag())
	assert.Equal(t, true, boolRequiredFlag())
}
