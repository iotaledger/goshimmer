package blocklayer

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

var acceptanceGadget *acceptance.Gadget

// AcceptanceGadget is the finality gadget instance.
func AcceptanceGadget() *acceptance.Gadget {
	return acceptanceGadget
}

func configureFinality() {
	deps.Tangle.ApprovalWeightManager.Events.MarkerWeightChanged.Attach(event.NewClosure(func(e *tangleold.MarkerWeightChangedEvent) {
		if err := acceptanceGadget.HandleMarker(e.Marker, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))
	deps.Tangle.ApprovalWeightManager.Events.ConflictWeightChanged.Attach(event.NewClosure(func(e *tangleold.ConflictWeightChangedEvent) {
		if err := acceptanceGadget.HandleConflict(e.ConflictID, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))

	// we need to update the WeightProvider on confirmation
	acceptanceGadget.Events().BlockAccepted.Attach(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		ei := epoch.IndexFromTime(event.Block.IssuingTime())
		deps.Tangle.WeightProvider.Update(ei, identity.NewID(event.Block.IssuerPublicKey()))
	}))
}
