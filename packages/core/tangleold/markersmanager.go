package tangleold

import (
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// region MarkersManager ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMarkersMapper is a Tangle component that takes care of managing the Markers which are used to infer structural
// information about the Tangle in an efficient way.
type ConflictMarkersMapper struct {
	tangle *Tangle
	*markers.Manager
}

// NewConflictMarkersMapper is the constructor of the MarkersManager.
func NewConflictMarkersMapper(tangle *Tangle) (b *ConflictMarkersMapper) {
	b = &ConflictMarkersMapper{
		tangle:  tangle,
		Manager: markers.NewManager(markers.WithStore(tangle.Options.Store)),
	}

	// Always set Genesis to MasterConflict.
	b.SetConflictIDs(markers.NewMarker(0, 0), set.NewAdvancedSet[utxo.TransactionID]())

	return
}

// InheritStructureDetails returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (b *ConflictMarkersMapper) InheritStructureDetails(block *Block, structureDetails []*markers.StructureDetails) (newStructureDetails *markers.StructureDetails, newSequenceCreated bool) {
	// newStructureDetails, newSequenceCreated = b.Manager.InheritStructureDetails(structureDetails, func(sequenceID markers.SequenceID, currentHighestIndex markers.Index) bool {
	//	nodeID := identity.NewID(block.IssuerPublicKey())
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
	if newStructureDetails.IsPastMarker() {
		b.SetBlockID(newStructureDetails.PastMarkers().Marker(), block.ID())
	}

	return
}

// BlockID retrieves the BlockID of the given Marker.
func (b *ConflictMarkersMapper) BlockID(marker markers.Marker) (blockID BlockID) {
	b.tangle.Storage.MarkerBlockMapping(marker).Consume(func(markerBlockMapping *MarkerBlockMapping) {
		blockID = markerBlockMapping.BlockID()
	})

	return
}

// SetBlockID associates a BlockID with the given Marker.
func (b *ConflictMarkersMapper) SetBlockID(marker markers.Marker, blockID BlockID) {
	b.tangle.Storage.StoreMarkerBlockMapping(NewMarkerBlockMapping(marker, blockID))
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (b *ConflictMarkersMapper) SetConflictIDs(marker markers.Marker, conflictIDs *set.AdvancedSet[utxo.TransactionID]) (updated bool) {
	if floorMarker, floorConflictIDs, exists := b.Floor(marker); exists {
		if floorConflictIDs.Equal(conflictIDs) {
			return false
		}

		if floorMarker == marker.Index() {
			b.deleteConflictIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	b.setConflictIDMapping(marker, b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(conflictIDs))

	return true
}

// ConflictIDs returns the ConflictID that is associated with the given Marker.
func (b *ConflictMarkersMapper) ConflictIDs(marker markers.Marker) (conflictIDs *set.AdvancedSet[utxo.TransactionID]) {
	b.tangle.Storage.MarkerIndexConflictIDMapping(marker.SequenceID()).Consume(func(markerIndexConflictIDMapping *MarkerIndexConflictIDMapping) {
		conflictIDs = markerIndexConflictIDMapping.ConflictIDs(marker.Index())
	})

	return
}

func (b *ConflictMarkersMapper) setConflictIDMapping(marker markers.Marker, conflictIDs *set.AdvancedSet[utxo.TransactionID]) bool {
	return b.tangle.Storage.MarkerIndexConflictIDMapping(marker.SequenceID(), NewMarkerIndexConflictIDMapping).Consume(func(markerIndexConflictIDMapping *MarkerIndexConflictIDMapping) {
		markerIndexConflictIDMapping.SetConflictIDs(marker.Index(), conflictIDs)
	})
}

func (b *ConflictMarkersMapper) deleteConflictIDMapping(marker markers.Marker) bool {
	return b.tangle.Storage.MarkerIndexConflictIDMapping(marker.SequenceID(), NewMarkerIndexConflictIDMapping).Consume(func(markerIndexConflictIDMapping *MarkerIndexConflictIDMapping) {
		markerIndexConflictIDMapping.DeleteConflictID(marker.Index())
	})
}

// Floor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (b *ConflictMarkersMapper) Floor(referenceMarker markers.Marker) (marker markers.Index, conflictIDs *set.AdvancedSet[utxo.TransactionID], exists bool) {
	b.tangle.Storage.MarkerIndexConflictIDMapping(referenceMarker.SequenceID(), NewMarkerIndexConflictIDMapping).Consume(func(markerIndexConflictIDMapping *MarkerIndexConflictIDMapping) {
		marker, conflictIDs, exists = markerIndexConflictIDMapping.Floor(referenceMarker.Index())
	})

	return
}

// Ceiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (b *ConflictMarkersMapper) Ceiling(referenceMarker markers.Marker) (marker markers.Index, conflictIDs *set.AdvancedSet[utxo.TransactionID], exists bool) {
	b.tangle.Storage.MarkerIndexConflictIDMapping(referenceMarker.SequenceID(), NewMarkerIndexConflictIDMapping).Consume(func(markerIndexConflictIDMapping *MarkerIndexConflictIDMapping) {
		marker, conflictIDs, exists = markerIndexConflictIDMapping.Ceiling(referenceMarker.Index())
	})

	return
}

// ForEachConflictIDMapping iterates over all ConflictID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (b *ConflictMarkersMapper) ForEachConflictIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker markers.Marker, mappedConflictIDs *set.AdvancedSet[utxo.TransactionID])) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = b.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedConflictIDs)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (b *ConflictMarkersMapper) ForEachMarkerReferencingMarker(referencedMarker markers.Marker, callback func(referencingMarker markers.Marker)) {
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

// increaseMarkersIndexCallbackStrategy implements the default strategy for increasing marker Indexes in the Tangle.
func increaseMarkersIndexCallbackStrategy(markers.SequenceID, markers.Index) bool {
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
