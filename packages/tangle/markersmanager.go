package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/walker"
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
	b.SetBranchIDs(markers.NewMarker(0, 0), ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))

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

// PendingBranchIDs returns the pending BranchIDs that are associated with the given Marker.
func (b *BranchMarkersMapper) PendingBranchIDs(marker *markers.Marker) (branchIDs ledgerstate.BranchIDs, err error) {
	if branchIDs, err = b.tangle.LedgerState.ResolvePendingBranchIDs(b.branchIDs(marker)); err != nil {
		err = errors.Errorf("failed to resolve pending BranchIDs of marker %s: %w", marker, err)
	}
	return
}

// SetBranchIDs associates ledgerstate.BranchIDs with the given Marker.
func (b *BranchMarkersMapper) SetBranchIDs(marker *markers.Marker, branchIDs ledgerstate.BranchIDs) (updated bool) {
	if floorMarker, floorBranchIDs, exists := b.Floor(marker); exists {
		if floorBranchIDs.Equals(branchIDs) {
			return false
		}

		if floorMarker == marker.Index() {
			b.deleteBranchIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	b.setBranchIDMapping(marker, branchIDs)

	return true
}

// branchIDs returns the BranchID that is associated with the given Marker.
func (b *BranchMarkersMapper) branchIDs(marker *markers.Marker) (branchIDs ledgerstate.BranchIDs) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID()).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		branchIDs = markerIndexBranchIDMapping.BranchIDs(marker.Index())
	})

	return
}

func (b *BranchMarkersMapper) setBranchIDMapping(marker *markers.Marker, branchIDs ledgerstate.BranchIDs) bool {
	return b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchIDs(marker.Index(), branchIDs)
	})
}

func (b *BranchMarkersMapper) deleteBranchIDMapping(marker *markers.Marker) bool {
	return b.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.DeleteBranchID(marker.Index())
	})
}

// Floor returns the largest Index that is <= the given Marker, it's BranchIDs and a boolean value indicating if it
// exists.
func (b *BranchMarkersMapper) Floor(referenceMarker *markers.Marker) (marker markers.Index, branchIDs ledgerstate.BranchIDs, exists bool) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchIDs, exists = markerIndexBranchIDMapping.Floor(referenceMarker.Index())
	})

	return
}

// Ceiling returns the smallest Index that is >= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (b *BranchMarkersMapper) Ceiling(referenceMarker *markers.Marker) (marker markers.Index, branchIDs ledgerstate.BranchIDs, exists bool) {
	b.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchIDs, exists = markerIndexBranchIDMapping.Ceiling(referenceMarker.Index())
	})

	return
}

// ForEachBranchIDMapping iterates over all BranchID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (b *BranchMarkersMapper) ForEachBranchIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker *markers.Marker, mappedBranchIDs ledgerstate.BranchIDs)) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedBranchIDs, exists := b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedBranchIDs, exists = b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedBranchIDs)
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
func (b *BranchMarkersMapper) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker[MessageID]) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker[MessageID]) {
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
				for strongParentMessageID := range message.ParentsByType(StrongParentType) {
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
