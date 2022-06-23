package confirmation

import (
	"github.com/iotaledger/hive.go/types/confirmation"
)

type Gadget struct {
	state         confirmation.State
	thresholdFunc func() uint64
}

func NewGadget(state confirmation.State) (newInstance *Gadget) {
	return &Gadget{state: state}
}

func NewConfirmationGadget() (newInstance *Gadget) {
	return NewGadget(confirmation.Pending)
}
