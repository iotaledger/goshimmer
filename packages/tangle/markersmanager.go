package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region MarkersManager ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchMarkersMapper is a Tangle component that takes care of managing the Markers which are used to infer structural
// information about the Tangle in an efficient way.
type BranchMarkersMapper struct {
	tangle         *Tangle
	discardedNodes map[identity.ID]time.Time
	*markers.Manager

	Events *BranchMarkersMapperEvents
}

// NewBranchMarkersMapper is the constructor of the MarkersManager.
func NewBranchMarkersMapper(tangle *Tangle) (b *BranchMarkersMapper) {
	b = &BranchMarkersMapper{
		tangle:         tangle,
		discardedNodes: make(map[identity.ID]time.Time),
		Manager:        markers.NewManager(markers.WithStore(tangle.Options.Store)),
		Events: &BranchMarkersMapperEvents{
			FutureMarkerUpdated: events.NewEvent(futureMarkerUpdateEventCaller),
		},
	}

	// Always set Genesis to MasterBranch.
	b.SetBranchID(markers.NewMarker(0, 0), ledgerstate.MasterBranchID)

	return
}

// InheritStructureDetails returns the structure Details of a Message that are derived from the StructureDetails of its
// strong and like parents.
func (b *BranchMarkersMapper) InheritStructureDetails(message *Message, structureDetails []*markers.StructureDetails) (newStructureDetails *markers.StructureDetails, newSequenceCreated bool) {
	// newStructureDetails, newSequenceCreated = b.Manager.InheritStructureDetails(structureDetails, func(sequenceID markers.SequenceID, currentHighestIndex markers.Index) bool {
	//	nodeID := identity.NewID(message.IssuerPublicKey())
	//	bufferUsedRatio := float64(b.tangle.Scheduler.BufferSize()) / float64(b.tangle.Scheduler.MaxBufferSize())
	//	nodeQueueRatio := float64(b.tangle.Scheduler.NodeQueueSize(nodeID)) / float64(b.tangle.Scheduler.BufferSize())
	//	if bufferUsedRatio > 0.01 && nodeQueueRatio > 0.1 {
	//		return false
	//	}
	//	if discardTime, ok := b.discardedNodes[nodeID]; ok && time.Since(discardTime) < time.Minute {
	//		return false
	//	} else if ok && time.Since(discardTime) >= time.Minute {
	//		delete(b.discardedNodes, nodeID)
	//	}
	//	return b.tangle.Options.IncreaseMarkersIndexCallback(sequenceID, currentHighestIndex)
	// })

	newStructureDetails, newSequenceCreated = b.Manager.InheritStructureDetails(structureDetails, b.tangle.Options.IncreaseMarkersIndexCallback)
	if newStructureDetails.IsPastMarker {
		b.SetMessageID(newStructureDetails.PastMarkers.Marker(), message.ID())
		b.tangle.Utils.WalkMessageMetadata(b.propagatePastMarkerToFutureMarkers(newStructureDetails.PastMarkers.Marker()), message.ParentsByType(StrongParentType))
	}

	return
}

// MessageID retrieves the MessageID of the given Marker.
func (b *BranchMarkersMapper) MessageID(marker *markers.Marker) (messageID MessageID) {
	b.tangle.Storage.MarkerMessageMapping(marker).Consume(func(markerMessageMapping *MarkerMessageMapping) {
		messageID = markerMessageMapping.MessageID()
	})

	return
}

// SetMessageID associates a MessageID with the given Marker.
func (b *BranchMarkersMapper) SetMessageID(marker *markers.Marker, messageID MessageID) {
	b.tangle.Storage.StoreMarkerMessageMapping(NewMarkerMessageMapping(marker, messageID))
}

// BranchID returns the BranchID that is associated with the given Marker.
func (b *BranchMarkersMapper) BranchID(marker *markers.Marker) (branchID ledgerstate.BranchID) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID()).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		branchID = markerIndexBranchIDMapping.BranchID(marker.Index())
	})

	return
}

// ConflictBranchIDs returns the ConflictBranchIDs that are associated with the given Marker.
func (b *BranchMarkersMapper) ConflictBranchIDs(marker *markers.Marker) (branchIDs ledgerstate.BranchIDs, err error) {
	if branchIDs, err = b.tangle.LedgerState.ResolvePendingConflictBranchIDs(ledgerstate.NewBranchIDs(b.BranchID(marker))); err != nil {
		err = errors.Errorf("failed to resolve ConflictBranchIDs of marker %s: %w", marker, err)
	}
	return
}

// SetBranchID associates a BranchID with the given Marker.
func (b *BranchMarkersMapper) SetBranchID(marker *markers.Marker, branchID ledgerstate.BranchID) (updated bool) {
	if floorMarker, floorBranchID, exists := b.Floor(marker); exists {
		if floorBranchID == branchID {
			return false
		}

		if floorMarker == marker.Index() {
			b.deleteBranchIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	b.setBranchIDMapping(marker, branchID)

	return true
}

func (b *BranchMarkersMapper) setBranchIDMapping(marker *markers.Marker, branchID ledgerstate.BranchID) bool {
	return b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchID(marker.Index(), branchID)
	})
}

func (b *BranchMarkersMapper) deleteBranchIDMapping(marker *markers.Marker) bool {
	return b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.DeleteBranchID(marker.Index())
	})
}

// Floor returns the largest Index that is <= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (b *BranchMarkersMapper) Floor(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Floor(referenceMarker.Index())
	})

	return
}

// Ceiling returns the smallest Index that is >= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (b *BranchMarkersMapper) Ceiling(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Ceiling(referenceMarker.Index())
	})

	return
}

// ForEachBranchIDMapping iterates over all BranchID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (b *BranchMarkersMapper) ForEachBranchIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker *markers.Marker, mappedBranchID ledgerstate.BranchID)) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedBranchID, exists := b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedBranchID, exists = b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedBranchID)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (b *BranchMarkersMapper) ForEachMarkerReferencingMarker(referencedMarker *markers.Marker, callback func(referencingMarker *markers.Marker)) {
	b.Sequence(referencedMarker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
			if referencingSequenceID == referencedMarker.SequenceID() {
				return true
			}

			callback(markers.NewMarker(referencingSequenceID, referencingIndex))

			return true
		})
	})
}

// propagatePastMarkerToFutureMarkers updates the FutureMarkers of the strong parents of a given message when a new
// PastMaster was assigned.
func (b *BranchMarkersMapper) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		updated, inheritFurther := b.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
		if updated {
			messageMetadata.SetModified(true)

			b.Events.FutureMarkerUpdated.Trigger(&FutureMarkerUpdate{
				ID:           messageMetadata.ID(),
				FutureMarker: b.MessageID(pastMarkerToInherit),
			})
		}
		if inheritFurther {
			b.tangle.Storage.Message(messageMetadata.ID()).Consume(func(message *Message) {
				for _, strongParentMessageID := range message.ParentsByType(StrongParentType) {
					walker.Push(strongParentMessageID)
				}
			})
		}
	}
}

// increaseMarkersIndexCallbackStrategy implements the default strategy for increasing marker Indexes in the Tangle.
func increaseMarkersIndexCallbackStrategy(markers.SequenceID, markers.Index) bool {
	return true
}

// BranchMarkersMapperEvents represents events happening in the BranchMarkersMapper.
type BranchMarkersMapperEvents struct {
	// FutureMarkerUpdated is triggered when a message's future marker is updated.
	FutureMarkerUpdated *events.Event
}

// FutureMarkerUpdate contains the messageID of the future marker of a message.
type FutureMarkerUpdate struct {
	ID           MessageID
	FutureMarker MessageID
}

func futureMarkerUpdateEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(fmUpdate *FutureMarkerUpdate))(params[0].(*FutureMarkerUpdate))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
