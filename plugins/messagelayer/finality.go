package messagelayer

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/consensus/finality"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func FinalityGadget() finality.Gadget {
	return deps.Tangle.ConfirmationOracle.(finality.Gadget)
}

func configureFinality() {
	deps.Tangle.ApprovalWeightManager.Events.MarkerWeightChanged.Attach(events.NewClosure(func(e *tangle.MarkerWeightChangedEvent) {
		if err := FinalityGadget().HandleMarker(e.Marker, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))
	deps.Tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(events.NewClosure(func(e *tangle.BranchWeightChangedEvent) {
		if err := FinalityGadget().HandleBranch(e.BranchID, e.Weight); err != nil {
			Plugin.LogError(err)
		}
	}))

	// we need to update the WeightProvider on confirmation
	FinalityGadget().Events().MessageConfirmed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			deps.Tangle.WeightProvider.Update(message.IssuingTime(), identity.NewID(message.IssuerPublicKey()))
		})
	}))
}
