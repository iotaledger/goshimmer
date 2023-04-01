package markermanager

import (
	"github.com/emirpasic/gods/maps/treemap"
	"github.com/emirpasic/gods/utils"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

type MarkerManager[IndexedID slot.IndexedID, MappedEntity slot.IndexedEntity[IndexedID]] struct {
	Events                       *Events
	SequenceManager              *markers.SequenceManager
	markerIndexConflictIDMapping *shrinkingmap.ShrinkingMap[markers.SequenceID, *MarkerIndexConflictIDMapping]

	markerBlockMapping         *shrinkingmap.ShrinkingMap[markers.Marker, MappedEntity]
	sequenceMarkersMapping     *shrinkingmap.ShrinkingMap[markers.SequenceID, *treemap.Map]
	markerBlockMappingEviction *shrinkingmap.ShrinkingMap[slot.Index, set.Set[markers.Marker]]

	sequenceLastUsed *shrinkingmap.ShrinkingMap[markers.SequenceID, slot.Index]
	sequenceEviction *shrinkingmap.ShrinkingMap[slot.Index, set.Set[markers.SequenceID]]

	SequenceMutex *syncutils.DAGMutex[markers.SequenceID]

	optsSequenceManager []options.Option[markers.SequenceManager]
}

func NewMarkerManager[IndexedID slot.IndexedID, MappedEntity slot.IndexedEntity[IndexedID]](opts ...options.Option[MarkerManager[IndexedID, MappedEntity]]) *MarkerManager[IndexedID, MappedEntity] {
	manager := options.Apply(&MarkerManager[IndexedID, MappedEntity]{
		Events:                     NewEvents(),
		markerBlockMapping:         shrinkingmap.New[markers.Marker, MappedEntity](),
		sequenceMarkersMapping:     shrinkingmap.New[markers.SequenceID, *treemap.Map](),
		markerBlockMappingEviction: shrinkingmap.New[slot.Index, set.Set[markers.Marker]](),

		markerIndexConflictIDMapping: shrinkingmap.New[markers.SequenceID, *MarkerIndexConflictIDMapping](),
		sequenceLastUsed:             shrinkingmap.New[markers.SequenceID, slot.Index](),
		sequenceEviction:             shrinkingmap.New[slot.Index, set.Set[markers.SequenceID]](),

		SequenceMutex: syncutils.NewDAGMutex[markers.SequenceID](),

		optsSequenceManager: make([]options.Option[markers.SequenceManager], 0),
	}, opts)
	manager.SequenceManager = markers.NewSequenceManager(manager.optsSequenceManager...)

	manager.SetConflictIDs(markers.NewMarker(0, 0), utxo.NewTransactionIDs())
	manager.registerSequenceEviction(slot.Index(0), markers.SequenceID(0))

	return manager
}

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager[IndexedID, MappedEntity]) ProcessBlock(block MappedEntity, newSequenceCreated bool, conflictIDs utxo.TransactionIDs, newStructureDetails *markers.StructureDetails) {
	if newStructureDetails.IsPastMarker() {
		m.SequenceMutex.Lock(newStructureDetails.PastMarkers().Marker().SequenceID())
		defer m.SequenceMutex.Unlock(newStructureDetails.PastMarkers().Marker().SequenceID())

		if newSequenceCreated {
			m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
		}

		// if all conflictIDs are the same, they are already stored for the sequence. There is no need to explicitly set conflictIDs for the marker,
		// because thresholdmap is lowerbound and the conflictIDs are automatically set for the new marker.
		if !m.ConflictIDs(newStructureDetails.PastMarkers().Marker()).Equal(conflictIDs) {
			m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
		}
		m.addMarkerBlockMapping(newStructureDetails.PastMarkers().Marker(), block)
	}

	newStructureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		m.registerSequenceEviction(block.ID().Index(), sequenceID)
		return true
	})
}

func (m *MarkerManager[IndexedID, MappedEntity]) Evict(index slot.Index) {
	m.evictMarkerBlockMapping(index)
	m.evictSequences(index)
}

func (m *MarkerManager[IndexedID, MappedEntity]) evictSequences(index slot.Index) {
	if sequenceSet, sequenceSetExists := m.sequenceEviction.Get(index); sequenceSetExists {
		m.sequenceEviction.Delete(index)

		sequenceSet.ForEach(func(sequenceID markers.SequenceID) {
			if lastUsed, exists := m.sequenceLastUsed.Get(sequenceID); exists && lastUsed <= index {
				m.SequenceMutex.Lock(sequenceID)
				defer m.SequenceMutex.Unlock(sequenceID)

				m.sequenceLastUsed.Delete(sequenceID)
				m.markerIndexConflictIDMapping.Delete(sequenceID)
				m.sequenceMarkersMapping.Delete(sequenceID)
				m.SequenceManager.Delete(sequenceID)

				m.Events.SequenceEvicted.Trigger(sequenceID)
			}
		})
	}
}

func (m *MarkerManager[IndexedID, MappedEntity]) evictMarkerBlockMapping(index slot.Index) {
	if markerSet, exists := m.markerBlockMappingEviction.Get(index); exists {
		m.markerBlockMappingEviction.Delete(index)

		markerSet.ForEach(func(marker markers.Marker) {
			m.SequenceMutex.Lock(marker.SequenceID())
			defer m.SequenceMutex.Unlock(marker.SequenceID())

			m.markerBlockMapping.Delete(marker)

			if markerIndexBlockMapping, mappingExists := m.sequenceMarkersMapping.Get(marker.SequenceID()); mappingExists {
				markerIndexBlockMapping.Remove(uint64(marker.Index()))

				if markerIndexBlockMapping.Empty() {
					m.sequenceMarkersMapping.Delete(marker.SequenceID())
				}
			}
		})
	}
}

func (m *MarkerManager[IndexedID, MappedEntity]) registerSequenceEviction(index slot.Index, sequenceID markers.SequenceID) {
	// add the sequence to the set of active sequences for the given slot.Index. This is append-only (record of the past).
	sequenceSet, _ := m.sequenceEviction.GetOrCreate(index, func() set.Set[markers.SequenceID] {
		return set.New[markers.SequenceID](true)
	})
	sequenceSet.Add(sequenceID)

	// update the slot.Index in which the sequence was last used (it's a rolling state in the present)
	m.sequenceLastUsed.Set(sequenceID, index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping /////////////////////////////////////////////////////////////////////////////////

// ConflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (m *MarkerManager[IndexedID, MappedEntity]) ConflictIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs) {
	structureDetailsConflictIDs = utxo.NewTransactionIDs()

	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		structureDetailsConflictIDs.AddAll(m.ConflictIDs(markers.NewMarker(sequenceID, index)))
		return true
	})

	return
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) SetConflictIDs(marker markers.Marker, conflictIDs utxo.TransactionIDs) bool {
	if floorMarker, floorConflictIDs, exists := m.conflictsFloor(marker); exists {
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
	mapping, _ := m.markerIndexConflictIDMapping.GetOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.SetConflictIDs(marker.Index(), conflictIDs)
}

func (m *MarkerManager[IndexedID, MappedEntity]) deleteConflictIDMapping(marker markers.Marker) {
	if mapping, exists := m.markerIndexConflictIDMapping.Get(marker.SequenceID()); exists {
		mapping.DeleteConflictID(marker.Index())
	}
}

// conflictsFloor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (m *MarkerManager[IndexedID, MappedEntity]) conflictsFloor(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Floor(referenceMarker.Index())
}

// conflictsCeiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (m *MarkerManager[IndexedID, MappedEntity]) conflictsCeiling(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
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
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := m.conflictsCeiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = m.conflictsCeiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
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

// BlockFloor returns the largest Index that is <= the given Marker and a boolean value indicating if it exists.
func (m *MarkerManager[IndexedID, MappedEntity]) BlockFloor(marker markers.Marker) (floorMarker markers.Marker, exists bool) {
	m.SequenceMutex.RLock(marker.SequenceID())
	defer m.SequenceMutex.RUnlock(marker.SequenceID())

	mapping, sequenceMappingExists := m.sequenceMarkersMapping.Get(marker.SequenceID())
	if !sequenceMappingExists {
		return
	}

	floorMarkerRaw, _ := mapping.Floor(uint64(marker.Index()))
	if floorMarkerRaw != nil {
		exists = true
		floorMarker = markers.NewMarker(marker.SequenceID(), markers.Index(floorMarkerRaw.(uint64)))
	}

	return floorMarker, exists
}

// BlockCeiling returns the smallest Index that is >= the given Marker and a boolean value indicating if it exists.
func (m *MarkerManager[IndexedID, MappedEntity]) BlockCeiling(marker markers.Marker) (ceilingMarker markers.Marker, exists bool) {
	m.SequenceMutex.RLock(marker.SequenceID())
	defer m.SequenceMutex.RUnlock(marker.SequenceID())

	mapping, sequenceMappingExists := m.sequenceMarkersMapping.Get(marker.SequenceID())
	if !sequenceMappingExists {
		return
	}

	ceilingMarkerRaw, _ := mapping.Ceiling(uint64(marker.Index()))
	if ceilingMarkerRaw != nil {
		exists = true
		ceilingMarker = markers.NewMarker(marker.SequenceID(), markers.Index(ceilingMarkerRaw.(uint64)))
	}

	return ceilingMarker, exists
}

// addMarkerBlockMapping associates a Block with the given Marker.
func (m *MarkerManager[IndexedID, MappedEntity]) addMarkerBlockMapping(marker markers.Marker, block MappedEntity) {
	m.markerBlockMapping.Set(marker, block)

	markerSet, _ := m.markerBlockMappingEviction.GetOrCreate(block.ID().Index(), func() set.Set[markers.Marker] { return set.New[markers.Marker](true) })
	markerSet.Add(marker)

	markerIndexBlockMapping, _ := m.sequenceMarkersMapping.GetOrCreate(marker.SequenceID(), func() *treemap.Map {
		return treemap.NewWith(utils.UInt64Comparator)
	})
	markerIndexBlockMapping.Put(uint64(marker.Index()), block)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSequenceManagerOptions[IndexedID slot.IndexedID, MappedEntity slot.IndexedEntity[IndexedID]](opts ...options.Option[markers.SequenceManager]) options.Option[MarkerManager[IndexedID, MappedEntity]] {
	return func(b *MarkerManager[IndexedID, MappedEntity]) {
		b.optsSequenceManager = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
