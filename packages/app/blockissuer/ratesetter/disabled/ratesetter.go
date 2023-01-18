package disabled

import (
	"time"
)

// region DisabledRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct{}

func New() *RateSetter {
	return &RateSetter{}
}

func (r *RateSetter) Shutdown() {
	// don't need to shut the disabled ratesetter down.
}

func (r *RateSetter) Rate() float64 {
	return 0
}
func (r *RateSetter) Estimate() time.Duration {
	return time.Duration(0)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
