package messagelayer

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var finalityGadget finality.Gadget

// FinalityGadget is the finality gadget instance.
func FinalityGadget() finality.Gadget {
	return finalityGadget
}

func configureFinality() {
	deps.Tangle.ApprovalWeightManager.Events.MarkerWeightChanged.Attach(event.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		if err := finalityGadget.HandleMarker(e.Marker, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))
	deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(event.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		if err := finalityGadget.HandleBranch(e.BranchID, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))

	// we need to update the WeightProvider on confirmation
	finalityGadget.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		deps.Tangle.Storage.Message(event.MessageID).Consume(func(message *tangle.Message) {
			deps.Tangle.WeightProvider.Update(message.IssuingTime(), identity.NewID(message.IssuerPublicKey()))
		})
	}))
}
