package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/hive.go/events"
)

type Instance struct {
	State  *state.State
	Events *events.Event
}

func New() *Instance {
	return &Instance{
		State: state.New(),
	}
}
