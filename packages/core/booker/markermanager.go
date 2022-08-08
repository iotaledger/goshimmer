package booker

import (
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

	lastUsedMap *memstorage.Storage[markers.SequenceID, epoch.Index]
	pruningMap  *memstorage.Storage[epoch.Index, set.Set[markers.SequenceID]]
}

func NewMarkerManager() *MarkerManager {
	return &MarkerManager{
		sequenceManager:              markers.NewSequenceManager(markers.WithMaxPastMarkerDistance(3)),
		markerIndexConflictIDMapping: memstorage.New[markers.SequenceID, *MarkerIndexConflictIDMapping](),
		lastUsedMap:                  memstorage.New[markers.SequenceID, epoch.Index](),
		pruningMap:                   memstorage.New[epoch.Index, set.Set[markers.SequenceID]](),
	}
}

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager) ProcessBlock(block *Block, structureDetails []*markers.StructureDetails, conflictIDs utxo.TransactionIDs) (newStructureDetails *markers.StructureDetails) {
	newStructureDetails, newSequenceCreated := m.sequenceManager.InheritStructureDetails(structureDetails)

	if newSequenceCreated {
		m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
	}

	if newStructureDetails.IsPastMarker() {
		m.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), conflictIDs)
		m.AddMarkerBlockMapping(newStructureDetails.PastMarkers().Marker(), block)
	}

	// TODO: register in pruning map and lastUsedMap

	return
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
// func (b *MarkerManager) ForEachMarkerReferencingMarker(referencedMarker markers.Marker, callback func(referencingMarker markers.Marker)) {
// 	b.Sequence(referencedMarker.SequenceID()).Consume(func(sequence *markers.Sequence) {
// 		sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
// 			if referencingSequenceID == referencedMarker.SequenceID() {
// 				return true
// 			}
//
// 			callback(markers.NewMarker(referencingSequenceID, referencingIndex))
//
// 			return true
// 		})
// 	})
// }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping /////////////////////////////////////////////////////////////////////////////////

// ConflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (m *MarkerManager) ConflictIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs) {
	structureDetailsConflictIDs = utxo.NewTransactionIDs()

	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		conflictIDs := m.ConflictIDs(markers.NewMarker(sequenceID, index))
		structureDetailsConflictIDs.AddAll(conflictIDs)
		return true
	})

	return
}

// SetConflictIDs associates ledger.ConflictIDs with the given Marker.
func (m *MarkerManager) SetConflictIDs(marker markers.Marker, conflictIDs *set.AdvancedSet[utxo.TransactionID]) (updated bool) {
	if floorMarker, floorConflictIDs, exists := m.Floor(marker); exists {
		if floorConflictIDs.Equal(conflictIDs) {
			return false
		}

		if floorMarker == marker.Index() {
			m.deleteConflictIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}
	}

	// Merge to master as we only get the unconfirmed conflict IDs.
	// TODO: m.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(conflictIDs)
	m.setConflictIDMapping(marker, conflictIDs)

	return true
}

// ConflictIDs returns the ConflictID that is associated with the given Marker.
func (m *MarkerManager) ConflictIDs(marker markers.Marker) (conflictIDs utxo.TransactionIDs) {
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

// Floor returns the largest Index that is <= the given Marker, it's ConflictIDs and a boolean value indicating if it
// exists.
func (m *MarkerManager) Floor(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Floor(referenceMarker.Index())
}

// Ceiling returns the smallest Index that is >= the given Marker, it's ConflictID and a boolean value indicating if it
// exists.
func (m *MarkerManager) Ceiling(referenceMarker markers.Marker) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	mapping, exists := m.markerIndexConflictIDMapping.Get(referenceMarker.SequenceID())
	if !exists {
		return
	}
	return mapping.Ceiling(referenceMarker.Index())
}

// ForEachConflictIDMapping iterates over all ConflictID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkerManager) ForEachConflictIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker markers.Marker, mappedConflictIDs *set.AdvancedSet[utxo.TransactionID])) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedConflictIDs, exists := m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedConflictIDs, exists = m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedConflictIDs)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerBlockMapping ///////////////////////////////////////////////////////////////////////////////////////////

// BlockFromMarker retrieves the Block of the given Marker.
func (m *MarkerManager) BlockFromMarker(marker markers.Marker) (block *Block, exists bool) {
	return m.markerBlockMapping.Get(marker)
}

// AddMarkerBlockMapping associates a Block with the given Marker.
func (m *MarkerManager) AddMarkerBlockMapping(marker markers.Marker, block *Block) {
	m.markerBlockMapping.Set(marker, block)
	markerSet, _ := m.markerBlockMappingPruning.RetrieveOrCreate(block.ID().EpochIndex, func() set.Set[markers.Marker] { return set.New[markers.Marker](true) })
	markerSet.Add(marker)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
