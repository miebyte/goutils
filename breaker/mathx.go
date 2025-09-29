package breaker

import (
	"math/rand"
	"sync"
	"time"

	"github.com/miebyte/goutils/utils"
)

// A Proba is used to test if true on given probability.
type Proba struct {
	// rand.New(...) returns a non thread safe object
	r    *rand.Rand
	lock sync.Mutex
}

// NewProba returns a Proba.
func NewProba() *Proba {
	return &Proba{
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// TrueOnProba checks if true on given probability.
func (p *Proba) TrueOnProba(proba float64) (truth bool) {
	p.lock.Lock()
	truth = p.r.Float64() < proba
	p.lock.Unlock()
	return
}

// AtLeast returns the greater of x or lower.
func AtLeast[T utils.Numerical](x, lower T) T {
	if x < lower {
		return lower
	}
	return x
}

// AtMost returns the smaller of x or upper.
func AtMost[T utils.Numerical](x, upper T) T {
	if x > upper {
		return upper
	}
	return x
}

// Between returns the value of x clamped to the range [lower, upper].
func Between[T utils.Numerical](x, lower, upper T) T {
	if x < lower {
		return lower
	}
	if x > upper {
		return upper
	}
	return x
}
