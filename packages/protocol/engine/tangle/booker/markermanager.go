package booker

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	markers2 "github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type MarkerManager struct {
	sequenceManager              *markers2.SequenceManager
	markerIndexConflictIDMapping *memstorage.Storage[markers2.SequenceID, *MarkerIndexConflictIDMapping]

	markerBlockMapping         *memstorage.Storage[markers2.Marker, *Block]
	markerBlockMappingEviction *memstorage.Storage[epoch.Index, set.Set[markers2.Marker]]

	sequenceLastUsed *memstorage.Storage[markers2.SequenceID, epoch.Index]
	sequenceEviction *memstorage.Storage[epoch.Index, set.Set[markers2.SequenceID]]

	optsSequenceManager []options.Option[markers2.SequenceManager]
}

func NewMarkerManager(opts ...options.Option[MarkerManager]) *MarkerManager {
	manager := options.Apply(&MarkerManager{
		markerBlockMapping:         memstorage.New[markers2.Marker, *Block](),
		markerBlockMappingEviction: memstorage.New[epoch.Index, set.Set[markers2.Marker]](),

		markerIndexConflictIDMapping: memstorage.New[markers2.SequenceID, *MarkerIndexConflictIDMapping](),
		sequenceLastUsed:             memstorage.New[markers2.SequenceID, epoch.Index](),
		sequenceEviction:             memstorage.New[epoch.Index, set.Set[markers2.SequenceID]](),
		optsSequenceManager:          make([]options.Option[markers2.SequenceManager], 0),
	}, opts)
	manager.sequenceManager = markers2.NewSequenceManager(manager.optsSequenceManager...)

	manager.SetConflictIDs(markers2.NewMarker(0, 0), utxo.NewTransactionIDs())
	manager.registerSequenceEviction(epoch.Index(0), markers2.SequenceID(0))

	return manager
}

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager) ProcessBlock(block *Block, structureDetails []*markers2.StructureDetails, conflictIDs utxo.TransactionIDs) (newStructureDetails *markers2.StructureDetails) {
	newStructureDetails, newSequenceCreated := m.sequenceManager.InheritStructureDetails(structureDetails)

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

func (m *MarkerManager) EvictEpoch(epochIndex epoch.Index) {
	m.evictMarkerBlockMapping(epochIndex)
	m.evictSequences(epochIndex)
}

func (m *MarkerManager) evictSequences(epochIndex epoch.Index) {
	if sequenceSet, sequenceSetExists := m.sequenceEviction.Get(epochIndex); sequenceSetExists {
		m.sequenceEviction.Delete(epochIndex)

		sequenceSet.ForEach(func(sequenceID markers2.SequenceID) {
			if lastUsed, exists := m.sequenceLastUsed.Get(sequenceID); exists && lastUsed <= epochIndex {
				m.sequenceLastUsed.Delete(sequenceID)
				m.markerIndexConflictIDMapping.Delete(sequenceID)
				m.sequenceManager.Delete(sequenceID)
			}
		})
	}
}

func (m *MarkerManager) evictMarkerBlockMapping(epochIndex epoch.Index) {
	if markerSet, exists := m.markerBlockMappingEviction.Get(epochIndex); exists {
		m.markerBlockMappingEviction.Delete(epochIndex)

		markerSet.ForEach(func(marker markers2.Marker) {
			m.markerBlockMapping.Delete(marker)
		})
	}
}

func (m *MarkerManager) registerSequenceEviction(index epoch.Index, sequenceID markers2.SequenceID) {
	// add the sequence to the set of active sequences for the given epoch.Index. This is append-only (record of the past).
	sequenceSet, _ := m.sequenceEviction.RetrieveOrCreate(index, func() set.Set[markers2.SequenceID] {
		return set.New[markers2.SequenceID](true)
	})
	sequenceSet.Add(sequenceID)

	// update the epoch.Index in which the sequence was last used (it's a rolling state in the present)
	m.sequenceLastUsed.Set(sequenceID, index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping /////////////////////////////////////////////////////////////////////////////////

// ConflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (m *MarkerManager) ConflictIDsFromStructureDetails(structureDetails *markers2.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs) {
	structureDetailsConflictIDs = utxo.NewTransactionIDs()

	structureDetails.PastMarkers().ForEach(func(sequenceID markers2.SequenceID, index markers2.Index) bool {
		conflictIDs := m.ConflictIDs(markers2.NewMarker(sequenceID, index))
		structureDetailsConflictIDs.AddAll(conflictIDs)
		return true
	})

	return
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (m *MarkerManager) SetConflictIDs(marker markers2.Marker, conflictIDs utxo.TransactionIDs) bool {
	if floorMarker, floorConflictIDs, exists := m.floor(marker); exists {
		if floorConflictIDs.Equal(conflictIDs) {
			return false
		}

		if floorMarker == marker.Index() {
			m.deleteConflictIDMapping(markers2.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	m.setConflictIDMapping(marker, conflictIDs)

	return true
}

// ConflictIDs returns the ConflictID that is associated with the given Marker.
func (m *MarkerManager) ConflictIDs(marker markers2.Marker) (conflictIDs utxo.TransactionIDs) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(marker.SequenceID())
	if !exists {
		return utxo.NewTransactionIDs()
	}

	return mapping.ConflictIDs(marker.Index())
}

func (m *MarkerManager) setConflictIDMapping(marker markers2.Marker, conflictIDs utxo.TransactionIDs) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.SetConflictIDs(marker.Index(), conflictIDs)
}

func (m *MarkerManager) deleteConflictIDMapping(marker markers2.Marker) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.DeleteConflictID(marker.Index())
}

// floor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (m *MarkerManager) floor(referenceMarker markers2.Marker) (marker markers2.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Floor(referenceMarker.Index())
}

// ceiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (m *MarkerManager) ceiling(referenceMarker markers2.Marker) (marker markers2.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Ceiling(referenceMarker.Index())
}

// ForEachConflictIDMapping iterates over all ConflictID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkerManager) ForEachConflictIDMapping(sequenceID markers2.SequenceID, thresholdIndex markers2.Index, callback func(mappedMarker markers2.Marker, mappedConflictIDs utxo.TransactionIDs)) {
	currentMarker := markers2.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := m.ceiling(markers2.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = m.ceiling(markers2.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers2.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedConflictIDs)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (m *MarkerManager) ForEachMarkerReferencingMarker(referencedMarker markers2.Marker, callback func(referencingMarker markers2.Marker)) {
	sequence, exists := m.sequenceManager.Sequence(referencedMarker.SequenceID())
	if !exists {
		return
	}

	sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers2.SequenceID, referencingIndex markers2.Index) bool {
		if referencingSequenceID == referencedMarker.SequenceID() {
			return true
		}

		callback(markers2.NewMarker(referencingSequenceID, referencingIndex))

		return true
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerBlockMapping ///////////////////////////////////////////////////////////////////////////////////////////

// BlockFromMarker retrieves the Block of the given Marker.
func (m *MarkerManager) BlockFromMarker(marker markers2.Marker) (block *Block, exists bool) {
	return m.markerBlockMapping.Get(marker)
}

// addMarkerBlockMapping associates a Block with the given Marker.
func (m *MarkerManager) addMarkerBlockMapping(marker markers2.Marker, block *Block) {
	m.markerBlockMapping.Set(marker, block)
	markerSet, _ := m.markerBlockMappingEviction.RetrieveOrCreate(block.ID().EpochIndex, func() set.Set[markers2.Marker] { return set.New[markers2.Marker](true) })
	markerSet.Add(marker)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSequenceManagerOptions(opts ...options.Option[markers2.SequenceManager]) options.Option[MarkerManager] {
	return func(b *MarkerManager) {
		b.optsSequenceManager = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
