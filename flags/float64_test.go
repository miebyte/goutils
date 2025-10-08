package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFloat64(t *testing.T) {
	float64Flag := Float64("float64", 10.0, "float64 flag")
	float64RequiredFlag := Float64Required("float64_required", "float64 required flag")

	sf.Set("float64", "10.0")
	sf.Set("float64_required", "13.14")

	assert.Equal(t, 10.0, float64Flag())
	assert.Equal(t, 13.14, float64RequiredFlag())
}
