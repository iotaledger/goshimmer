package blocklayer

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/core/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

var acceptanceGadget *acceptance.Gadget

// AcceptanceGadget is the finality gadget instance.
func AcceptanceGadget() *acceptance.Gadget {
	return acceptanceGadget
}

func configureFinality() {
	deps.Tangle.ApprovalWeightManager.Events.MarkerWeightChanged.Attach(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		if err := acceptanceGadget.HandleMarker(e.Marker, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))
	deps.Tangle.ApprovalWeightManager.Events.ConflictWeightChanged.Attach(event.NewClosure(func(e *tangle.ConflictWeightChangedEvent) {
		if err := acceptanceGadget.HandleConflict(e.ConflictID, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))

	// we need to update the WeightProvider on confirmation
	acceptanceGadget.Events().BlockAccepted.Attach(event.NewClosure(func(event *tangle.BlockAcceptedEvent) {
		deps.Tangle.WeightProvider.Update(event.Block.IssuingTime(), identity.NewID(event.Block.IssuerPublicKey()))
	}))
}
