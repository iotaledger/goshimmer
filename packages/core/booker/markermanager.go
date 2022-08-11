package booker

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// TODO: create MarkerManager component
//  - manages pruning of markers and all related (conflict mapping) entities

type MarkerManager struct {
	sequenceManager              *markers.SequenceManager
	markerIndexConflictIDMapping *memstorage.Storage[markers.SequenceID, *MarkerIndexConflictIDMapping]

	markerBlockMapping        *memstorage.Storage[markers.Marker, *Block]
	markerBlockMappingPruning *memstorage.Storage[epoch.Index, set.Set[markers.Marker]]

	sequenceLastUsed *memstorage.Storage[markers.SequenceID, epoch.Index]
	sequencePruning  *memstorage.Storage[epoch.Index, set.Set[markers.SequenceID]]

	// TODO: finish shutdown implementation, maybe replace with component state
	// maxDroppedEpoch contains the highest epoch.Index that has been dropped from the Tangle.
	maxDroppedEpoch epoch.Index

	pruningMutex sync.RWMutex
}

func NewMarkerManager() *MarkerManager {
	manager := &MarkerManager{
		sequenceManager: markers.NewSequenceManager(markers.WithMaxPastMarkerDistance(3)),

		markerBlockMapping:        memstorage.New[markers.Marker, *Block](),
		markerBlockMappingPruning: memstorage.New[epoch.Index, set.Set[markers.Marker]](),

		markerIndexConflictIDMapping: memstorage.New[markers.SequenceID, *MarkerIndexConflictIDMapping](),
		sequenceLastUsed:             memstorage.New[markers.SequenceID, epoch.Index](),
		sequencePruning:              memstorage.New[epoch.Index, set.Set[markers.SequenceID]](),
	}

	manager.SetConflictIDs(markers.NewMarker(0, 0), utxo.NewTransactionIDs())

	return manager
}

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager) ProcessBlock(block *Block, structureDetails []*markers.StructureDetails, conflictIDs utxo.TransactionIDs) (newStructureDetails *markers.StructureDetails) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()
	newStructureDetails, newSequenceCreated := m.sequenceManager.InheritStructureDetails(structureDetails)

	if newSequenceCreated {
		m.setConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
	}

	if newStructureDetails.IsPastMarker() {
		// if all conflictIDs are the same, they are already stored for the sequence. There is no need to explicitly set conflictIDs for the marker,
		// because thresholdmap is lowerbound and the conflictIDs are automatically set for the new marker.
		if !m.ConflictIDs(newStructureDetails.PastMarkers().Marker()).Equal(conflictIDs) {
			m.setConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
		}
		m.addMarkerBlockMapping(newStructureDetails.PastMarkers().Marker(), block)

		m.registerSequencePruning(block, newStructureDetails)
	}

	return
}

func (m *MarkerManager) Prune(epochIndex epoch.Index) {
	m.pruningMutex.Lock()
	defer m.pruningMutex.Unlock()

	for m.maxDroppedEpoch < epochIndex {
		m.maxDroppedEpoch++

		m.pruneMarkerBlockMapping()
		m.pruneSequences()
	}
}

func (m *MarkerManager) pruneSequences() {
	if sequenceSet, sequenceSetExists := m.sequencePruning.Get(m.maxDroppedEpoch); sequenceSetExists {
		m.sequencePruning.Delete(m.maxDroppedEpoch)

		sequenceSet.ForEach(func(sequenceID markers.SequenceID) {
			if lastUsed, exists := m.sequenceLastUsed.Get(sequenceID); exists && lastUsed <= m.maxDroppedEpoch {
				m.sequenceLastUsed.Delete(sequenceID)
				m.markerIndexConflictIDMapping.Delete(sequenceID)
				m.sequenceManager.Delete(sequenceID)
			}
		})
	}
}

func (m *MarkerManager) pruneMarkerBlockMapping() {
	if markerSet, exists := m.markerBlockMappingPruning.Get(m.maxDroppedEpoch); exists {
		m.markerBlockMappingPruning.Delete(m.maxDroppedEpoch)

		markerSet.ForEach(func(marker markers.Marker) {
			m.markerBlockMapping.Delete(marker)
		})
	}
}

func (m *MarkerManager) registerSequencePruning(block *Block, newStructureDetails *markers.StructureDetails) {
	blockSequenceID := newStructureDetails.PastMarkers().Marker().SequenceID()

	// add the sequence to the set of active sequences for the given epoch.Index. This is append only (record of the past).
	sequenceSet, _ := m.sequencePruning.RetrieveOrCreate(block.ID().Index(), func() set.Set[markers.SequenceID] {
		return set.New[markers.SequenceID](true)
	})
	sequenceSet.Add(blockSequenceID)

	// update the epoch.Index in which the sequence was last used (it's a rolling state in the present)
	m.sequenceLastUsed.Set(blockSequenceID, block.ID().Index())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping /////////////////////////////////////////////////////////////////////////////////

// ConflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (m *MarkerManager) ConflictIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	structureDetailsConflictIDs = utxo.NewTransactionIDs()

	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		conflictIDs := m.conflictIDs(markers.NewMarker(sequenceID, index))
		structureDetailsConflictIDs.AddAll(conflictIDs)
		return true
	})

	return
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (m *MarkerManager) SetConflictIDs(marker markers.Marker, conflictIDs *set.AdvancedSet[utxo.TransactionID]) (updated bool) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	return m.setConflictIDs(marker, conflictIDs)
}

func (m *MarkerManager) setConflictIDs(marker markers.Marker, conflictIDs *set.AdvancedSet[utxo.TransactionID]) bool {
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
func (m *MarkerManager) ConflictIDs(marker markers.Marker) (conflictIDs utxo.TransactionIDs) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	return m.conflictIDs(marker)
}

func (m *MarkerManager) conflictIDs(marker markers.Marker) utxo.TransactionIDs {
	mapping, exists := m.markerIndexConflictIDMapping.Get(marker.SequenceID())
	if !exists {
		return utxo.NewTransactionIDs()
	}

	return mapping.ConflictIDs(marker.Index())
}

func (m *MarkerManager) setConflictIDMapping(marker markers.Marker, conflictIDs utxo.TransactionIDs) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.SetConflictIDs(marker.Index(), conflictIDs)
}

func (m *MarkerManager) deleteConflictIDMapping(marker markers.Marker) {
	mapping, _ := m.markerIndexConflictIDMapping.RetrieveOrCreate(marker.SequenceID(), NewMarkerIndexConflictIDMapping)
	mapping.DeleteConflictID(marker.Index())
}

// floor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (m *MarkerManager) floor(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Floor(referenceMarker.Index())
}

// ceiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (m *MarkerManager) ceiling(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Ceiling(referenceMarker.Index())
}

// ForEachConflictIDMapping iterates over all ConflictID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkerManager) ForEachConflictIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker markers.Marker, mappedConflictIDs *set.AdvancedSet[utxo.TransactionID])) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := m.ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = m.ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedConflictIDs)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (m *MarkerManager) ForEachMarkerReferencingMarker(referencedMarker markers.Marker, callback func(referencingMarker markers.Marker)) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	sequence, exists := m.sequenceManager.Sequence(referencedMarker.SequenceID())
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
func (m *MarkerManager) BlockFromMarker(marker markers.Marker) (block *Block, exists bool) {
	m.pruningMutex.RLock()
	defer m.pruningMutex.RUnlock()

	return m.markerBlockMapping.Get(marker)
}

// addMarkerBlockMapping associates a Block with the given Marker.
func (m *MarkerManager) addMarkerBlockMapping(marker markers.Marker, block *Block) {
	m.markerBlockMapping.Set(marker, block)
	markerSet, _ := m.markerBlockMappingPruning.RetrieveOrCreate(block.ID().EpochIndex, func() set.Set[markers.Marker] { return set.New[markers.Marker](true) })
	markerSet.Add(marker)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
