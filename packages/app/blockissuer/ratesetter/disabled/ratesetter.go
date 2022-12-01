package disabled

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/utils"
)

// region DisabledRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	events *utils.Events
}

func New() *RateSetter {
	return &RateSetter{
		events: utils.NewEvents(),
	}
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
func (r *RateSetter) Events() *utils.Events {
	return r.events
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
