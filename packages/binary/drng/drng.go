package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
)

type Instance struct {
	State  *state.State
	Events *Event
}

func New(setters ...state.Option) *Instance {
	return &Instance{
		State:  state.New(setters...),
		Events: NewEvent(),
	}
}
