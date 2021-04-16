package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
)

func configureApprovalWeight() {
	Tangle().ApprovalWeightManager.Events.MarkerConfirmed.Attach(events.NewClosure(onMarkerConfirmed))
	// TODO: detect reorgs
}

func onMarkerConfirmed(marker *markers.Marker) {
	// get message ID of marker
	messageID := Tangle().Booker.MarkersManager.MessageID(marker)

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
