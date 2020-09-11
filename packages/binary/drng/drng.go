package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
)

// DRNG holds the state and events of a drng instance.
type DRNG struct {
	State  *state.State // The state of the DRNG.
	Events *Event       // The events fired on the DRNG.
}

// New creates a new DRNG instance.
func New(setters ...state.Option) *DRNG {
	return &DRNG{
		State:  state.New(setters...),
		Events: newEvent(),
	}
}
