package messagelayer

import (
	"fmt"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func configureApprovalWeight() {
	Tangle().ApprovalWeightManager.Events.MarkerConfirmation.Attach(events.NewClosure(onMarkerConfirmed))
	Tangle().ApprovalWeightManager.Events.BranchConfirmation.Attach(events.NewClosure(onBranchConfirmed))
}

func onMarkerConfirmed(marker markers.Marker, newLevel int, transition events.ThresholdEventTransition) {
	if transition != events.ThresholdLevelIncreased {
		plugin.LogInfo("transition != events.ThresholdLevelIncreased for %s", marker)
		return
	}
	// get message ID of marker
	messageID := Tangle().Booker.MarkersManager.MessageID(&marker)

	Tangle().Utils.WalkMessageAndMetadata(propagateFinalizedApprovalWeight, tangle.MessageIDs{messageID}, false)
}

func propagateFinalizedApprovalWeight(message *tangle.Message, messageMetadata *tangle.MessageMetadata, finalizedWalker *walker.Walker) {
	// stop walking to past cone if reach a marker
	if messageMetadata.StructureDetails().IsPastMarker && messageMetadata.IsFinalized() {
		return
	}

	// abort if the message is already finalized
	if !setMessageFinalized(messageMetadata) {
		return
	}

	// mark weak parents as finalized but not propagate finalized flag to its past cone
	message.ForEachWeakParent(func(parentID tangle.MessageID) {
		setPayloadFinalized(parentID)
	})

	// propagate finalized to strong parents
	message.ForEachStrongParent(func(parentID tangle.MessageID) {
		finalizedWalker.Push(parentID)
	})
}

func onBranchConfirmed(branchID ledgerstate.BranchID, newLevel int, transition events.ThresholdEventTransition) {
	if transition != events.ThresholdLevelIncreased {
		plugin.LogInfo("transition != events.ThresholdLevelIncreased for %s", branchID)
		return
	}

	_, err := Tangle().LedgerState.BranchDAG.SetBranchMonotonicallyLiked(branchID, true)
	if err != nil {
		panic(err)
	}
	_, err = Tangle().LedgerState.BranchDAG.SetBranchFinalized(branchID, true)
	if err != nil {
		panic(err)
	}

	if Tangle().LedgerState.BranchDAG.InclusionState(branchID) == ledgerstate.Rejected {
		remotelog.SendLogMsg(logger.LevelWarn, "REORG", fmt.Sprintf("%s reorg detected by ApprovalWeight", branchID))
		plugin.LogInfof("%s reorg detected by ApprovalWeight.", branchID)
	}
}

func setMessageFinalized(messageMetadata *tangle.MessageMetadata) (modified bool) {
	// abort if the message is already finalized
	if modified = messageMetadata.SetFinalized(true); !modified {
		return
	}

	Tangle().Storage.Message(messageMetadata.ID()).Consume(func(message *tangle.Message) {
		Tangle().WeightProvider.Update(message.IssuingTime(), identity.NewID(message.IssuerPublicKey()))
	})

	setPayloadFinalized(messageMetadata.ID())

	Tangle().ApprovalWeightManager.Events.MessageFinalized.Trigger(messageMetadata.ID())

	return modified
}

func setPayloadFinalized(messageID tangle.MessageID) {
	Tangle().Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		if err := Tangle().LedgerState.UTXODAG.SetTransactionConfirmed(transactionID); err != nil {
			plugin.LogError(err)
		}
	})
}
