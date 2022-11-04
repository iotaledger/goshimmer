package markermanager

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type MarkerManager[IndexedID epoch.IndexedID, MappedEntity epoch.IndexedEntity[IndexedID]] struct {
	Events                       *Events
	SequenceManager              *markers.SequenceManager
	markerIndexConflictIDMapping *memstorage.Storage[markers.SequenceID, *MarkerIndexConflictIDMapping]

	markerBlockMapping         *memstorage.Storage[markers.Marker, MappedEntity]
	markerBlockMappingEviction *memstorage.Storage[epoch.Index, set.Set[markers.Marker]]

	sequenceLastUsed *memstorage.Storage[markers.SequenceID, epoch.Index]
	sequenceEviction *memstorage.Storage[epoch.Index, set.Set[markers.SequenceID]]

	optsSequenceManager []options.Option[markers.SequenceManager]
}

func NewMarkerManager[IndexedID epoch.IndexedID, MappedEntity epoch.IndexedEntity[IndexedID]](opts ...options.Option[MarkerManager[IndexedID, MappedEntity]]) *MarkerManager[IndexedID, MappedEntity] {
	manager := options.Apply(&MarkerManager[IndexedID, MappedEntity]{
		Events:                     NewEvents(),
		markerBlockMapping:         memstorage.New[markers.Marker, MappedEntity](),
		markerBlockMappingEviction: memstorage.New[epoch.Index, set.Set[markers.Marker]](),

		markerIndexConflictIDMapping: memstorage.New[markers.SequenceID, *MarkerIndexConflictIDMapping](),
		sequenceLastUsed:             memstorage.New[markers.SequenceID, epoch.Index](),
		sequenceEviction:             memstorage.New[epoch.Index, set.Set[markers.SequenceID]](),
		optsSequenceManager:          make([]options.Option[markers.SequenceManager], 0),
	}, opts)
	manager.SequenceManager = markers.NewSequenceManager(manager.optsSequenceManager...)

	manager.SetConflictIDs(markers.NewMarker(0, 0), utxo.NewTransactionIDs())
	manager.registerSequenceEviction(epoch.Index(0), markers.SequenceID(0))

	return manager
}

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager[IndexedID, MappedEntity]) ProcessBlock(block MappedEntity, structureDetails []*markers.StructureDetails, conflictIDs utxo.TransactionIDs) (newStructureDetails *markers.StructureDetails) {
	newStructureDetails, newSequenceCreated := m.SequenceManager.InheritStructureDetails(structureDetails)

	if newSequenceCreated {
		m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
	}

	if newStructureDetails.IsPastMarker() {
		// if all conflictIDs are the same, they are already stored for the sequence. There is no need to explicitly set conflictIDs for the marker,
		// because thresholdmap is lowerbound and the conflictIDs are automatically set for the new marker.
		if !m.ConflictIDs(newStructureDetails.PastMarkers().Marker()).Equal(conflictIDs) {
			m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
		}
		m.addMarkerBlockMapping(newStructureDetails.PastMarkers().Marker(), block)

		m.registerSequenceEviction(block.ID().Index(), newStructureDetails.PastMarkers().Marker().SequenceID())
	}

	return
}

func (m *MarkerManager[IndexedID, MappedEntity]) EvictEpoch(epochIndex epoch.Index) {
	m.evictMarkerBlockMapping(epochIndex)
	m.evictSequences(epochIndex)
}

func (m *MarkerManager[IndexedID, MappedEntity]) evictSequences(epochIndex epoch.Index) {
	if sequenceSet, sequenceSetExists := m.sequenceEviction.Get(epochIndex); sequenceSetExists {
		m.sequenceEviction.Delete(epochIndex)

		sequenceSet.ForEach(func(sequenceID markers.SequenceID) {
			lastUsed, exists := m.sequenceLastUsed.Get(sequenceID)
			fmt.Println("iterating sequences", epochIndex, sequenceID, lastUsed, exists)
			if lastUsed, exists := m.sequenceLastUsed.Get(sequenceID); exists && lastUsed <= epochIndex {
				fmt.Println("delete sequence ", sequenceID)
				m.sequenceLastUsed.Delete(sequenceID)
				m.markerIndexConflictIDMapping.Delete(sequenceID)
				m.SequenceManager.Delete(sequenceID)

				m.Events.SequenceEvicted.Trigger(sequenceID)
			}
		})
	}
}

func (m *MarkerManager[IndexedID, MappedEntity]) evictMarkerBlockMapping(epochIndex epoch.Index) {
	if markerSet, exists := m.markerBlockMappingEviction.Get(epochIndex); exists {
		m.markerBlockMappingEviction.Delete(epochIndex)

		markerSet.ForEach(func(marker markers.Marker) {
			m.markerBlockMapping.Delete(marker)
		})
	}
}

func (m *MarkerManager[IndexedID, MappedEntity]) registerSequenceEviction(index epoch.Index, sequenceID markers.SequenceID) {
	// add the sequence to the set of active sequences for the given epoch.Index. This is append-only (record of the past).
	sequenceSet, _ := m.sequenceEviction.RetrieveOrCreate(index, func() set.Set[markers.SequenceID] {
		return set.New[markers.SequenceID](true)
	})
	sequenceSet.Add(sequenceID)

	// update the epoch.Index in which the sequence was last used (it's a rolling state in the present)
	m.sequenceLastUsed.Set(sequenceID, index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping /////////////////////////////////////////////////////////////////////////////////

// ConflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (m *MarkerManager[IndexedID, MappedEntity]) ConflictIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs) {
	structureDetailsConflictIDs = utxo.NewTransactionIDs()

	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		conflictIDs := m.ConflictIDs(markers.NewMarker(sequenceID, index))
		structureDetailsConflictIDs.AddAll(conflictIDs)
		return true
	})

	return
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) SetConflictIDs(marker markers.Marker, conflictIDs utxo.TransactionIDs) bool {
	if floorMarker, floorConflictIDs, exists := m.floor(marker); exists {
		if floorConflictIDs.Equal(conflictIDs) {
			return false
		}

		if floorMarker == marker.Index() {
			m.deleteConflictIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	m.setConflictIDMapping(marker, conflictIDs)

	return true
}

// ConflictIDs returns the ConflictID that is associated with the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) ConflictIDs(marker markers.Marker) (conflictIDs utxo.TransactionIDs) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(marker.SequenceID())
	if !exists {
		return utxo.NewTransactionIDs()
	}

	return mapping.ConflictIDs(marker.Index())
}

func (m *MarkerManager[IndexedID, MappedEntity]) setConflictIDMapping(marker markers.Marker, conflictIDs utxo.TransactionIDs) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.SetConflictIDs(marker.Index(), conflictIDs)
}

func (m *MarkerManager[IndexedID, MappedEntity]) deleteConflictIDMapping(marker markers.Marker) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.DeleteConflictID(marker.Index())
}

// floor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (m *MarkerManager[IndexedID, MappedEntity]) floor(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Floor(referenceMarker.Index())
}

// ceiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (m *MarkerManager[IndexedID, MappedEntity]) ceiling(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Ceiling(referenceMarker.Index())
}

// ForEachConflictIDMapping iterates over all ConflictID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkerManager[IndexedID, MappedEntity]) ForEachConflictIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker markers.Marker, mappedConflictIDs utxo.TransactionIDs)) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := m.ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = m.ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedConflictIDs)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) ForEachMarkerReferencingMarker(referencedMarker markers.Marker, callback func(referencingMarker markers.Marker)) {
	sequence, exists := m.SequenceManager.Sequence(referencedMarker.SequenceID())
	if !exists {
		return
	}

	sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
		if referencingSequenceID == referencedMarker.SequenceID() {
			return true
		}

		callback(markers.NewMarker(referencingSequenceID, referencingIndex))

		return true
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerBlockMapping ///////////////////////////////////////////////////////////////////////////////////////////

// BlockFromMarker retrieves the Block of the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) BlockFromMarker(marker markers.Marker) (block MappedEntity, exists bool) {
	block, exists = m.markerBlockMapping.Get(marker)
	return
}

// addMarkerBlockMapping associates a Block with the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) addMarkerBlockMapping(marker markers.Marker, block MappedEntity) {
	m.markerBlockMapping.Set(marker, block)
	markerSet, _ := m.markerBlockMappingEviction.RetrieveOrCreate(block.ID().Index(), func() set.Set[markers.Marker] { return set.New[markers.Marker](true) })
	markerSet.Add(marker)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSequenceManagerOptions[IndexedID epoch.IndexedID, MappedEntity epoch.IndexedEntity[IndexedID]](opts ...options.Option[markers.SequenceManager]) options.Option[MarkerManager[IndexedID, MappedEntity]] {
	return func(b *MarkerManager[IndexedID, MappedEntity]) {
		b.optsSequenceManager = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
