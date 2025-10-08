package flags

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuration(t *testing.T) {
	durationFlag := Duration("duration", 10*time.Second, "duration flag")
	durationRequiredFlag := DurationRequired("duration_required", "duration required flag")

	sf.Set("duration", "10s")
	sf.Set("duration_required", "10s")

	assert.Equal(t, 10*time.Second, durationFlag())
	assert.Equal(t, 10*time.Second, durationRequiredFlag())
}
