package drng

import (
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

func (drng *Instance) Dispatch(subtype header.Type, tx *transaction.Transaction) {
	switch subtype {
	case header.CollectiveBeaconType():
		// process collectiveBeacon
		if err := collectiveBeacon.ProcessTransaction(drng.State, drng.Events.CollectiveBeacon, tx); err != nil {
			return
		}
		// trigger RandomnessEvent
		drng.Events.Randomness.Trigger(drng.State.Randomness())

	default:
		//do other stuff
	}
}
