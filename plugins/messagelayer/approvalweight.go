package messagelayer

import (
	"fmt"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
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
		Plugin().LogInfo("transition != events.ThresholdLevelIncreased")
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
	if !setMessageFinalized(message, messageMetadata) {
		return
	}

	// mark weak parents as finalized but not propagate finalized flag to its past cone
	message.ForEachWeakParent(func(parentID tangle.MessageID) {
		Tangle().Storage.Message(parentID).Consume(func(parentMessage *tangle.Message) {
			Tangle().Storage.MessageMetadata(parentID).Consume(func(parentMetadata *tangle.MessageMetadata) {
				setMessageFinalized(parentMessage, parentMetadata)
			})
		})
	})

	// propagate finalized to strong parents
	message.ForEachStrongParent(func(parentID tangle.MessageID) {
		finalizedWalker.Push(parentID)
	})
}

func onBranchConfirmed(branchID ledgerstate.BranchID, newLevel int, transition events.ThresholdEventTransition) {
	plugin.LogDebugf("%s confirmed by ApprovalWeight.", branchID)

	Tangle().LedgerState.BranchDAG.SetBranchMonotonicallyLiked(branchID, true)
	Tangle().LedgerState.BranchDAG.SetBranchFinalized(branchID, true)

	if Tangle().LedgerState.BranchDAG.InclusionState(branchID) == ledgerstate.Rejected {
		remotelog.SendLogMsg(logger.LevelWarn, "REORG", fmt.Sprintf("%s reorg detected by ApprovalWeight", branchID))
		plugin.LogInfof("%s reorg detected by ApprovalWeight.", branchID)
	}
}

func setMessageFinalized(message *tangle.Message, messageMetadata *tangle.MessageMetadata) (modified bool) {
	// abort if the message is already finalized
	if modified = messageMetadata.SetFinalized(true); !modified {
		return
	}

	Tangle().Utils.ComputeIfTransaction(message.ID(), func(transactionID ledgerstate.TransactionID) {
		Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			modified := transactionMetadata.SetFinalized(true)
			if modified {
				Tangle().ConsensusManager.SetTransactionLiked(transactionID, true)
				// trigger TransactionOpinionFormed if the message contains a transaction
				Tangle().ConsensusManager.Events.TransactionConfirmed.Trigger(message.ID())
			}
		})
	})

	return
}
