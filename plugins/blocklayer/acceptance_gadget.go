package blocklayer

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
	deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		if err := acceptanceGadget.HandleBranch(e.BranchID, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))

	// we need to update the WeightProvider on confirmation
	acceptanceGadget.Events().BlockAccepted.Attach(event.NewClosure(func(event *tangle.BlockAcceptedEvent) {
		deps.Tangle.WeightProvider.Update(event.Block.IssuingTime(), identity.NewID(event.Block.IssuerPublicKey()))
	}))
}
