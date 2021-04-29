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

	// mark marker as finalized
	Tangle().Storage.MessageMetadata(messageID).Consume(func(metadata *tangle.MessageMetadata) {
		metadata.SetFinalizedApprovalWeight(true)
	})

	var entryMessageIDs tangle.MessageIDs
	Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		// mark weak parents as finalized but not propagate finalized flag to its past cone
		message.ForEachWeakParent(func(parentID tangle.MessageID) {
			Tangle().Storage.MessageMetadata(parentID).Consume(func(metadata *tangle.MessageMetadata) {
				metadata.SetFinalizedApprovalWeight(true)
			})
		})

		// propagate finalized flag to strong parents' past cone
		message.ForEachStrongParent(func(parentID tangle.MessageID) {
			entryMessageIDs = append(entryMessageIDs, parentID)
		})
	})

	Tangle().Utils.WalkMessageAndMetadata(propagateFinalizedApprovalWeight, entryMessageIDs, false)
}

func propagateFinalizedApprovalWeight(message *tangle.Message, messageMetadata *tangle.MessageMetadata, finalizedWalker *walker.Walker) {
	// stop walking to past cone if reach a marker
	if messageMetadata.StructureDetails().IsPastMarker {
		return
	}

	// abort if the message is already finalized
	if !messageMetadata.SetFinalizedApprovalWeight(true) {
		return
	}

	// mark weak parents as finalized but not propagate finalized flag to its past cone
	message.ForEachWeakParent(func(parentID tangle.MessageID) {
		Tangle().Storage.MessageMetadata(parentID).Consume(func(metadata *tangle.MessageMetadata) {
			metadata.SetFinalizedApprovalWeight(true)
		})
	})

	// propagate finalized to strong parents
	message.ForEachStrongParent(func(parentID tangle.MessageID) {
		finalizedWalker.Push(parentID)
	})
}

func onBranchConfirmed(branchID ledgerstate.BranchID, newLevel int, transition events.ThresholdEventTransition) {
	plugin.LogDebugf("%s confirmed by ApprovalWeight.", branchID)

	if Tangle().LedgerState.BranchDAG.InclusionState(branchID) == ledgerstate.Rejected {
		remotelog.SendLogMsg(logger.LevelWarn, "REORG", fmt.Sprintf("%s reorg detected by ApprovalWeight", branchID))
		plugin.LogInfof("%s reorg detected by ApprovalWeight.", branchID)
	}
}
