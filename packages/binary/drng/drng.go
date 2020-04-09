package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
)

type Instance struct {
	State  *state.State
	Events *Event
}

// New creates a new DRNG instance.
func New(setters ...state.Option) *Instance {
	return &Instance{
		State:  state.New(setters...),
		Events: NewEvent(),
	}
}
