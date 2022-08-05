package booker

import (
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/marker"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type MarkerManager struct {
	sequenceManager *marker.SequenceManager

	lastUsedMap *memstorage.Storage[marker.SequenceID, epoch.Index]
	pruningMap  *memstorage.Storage[epoch.Index, set.Set[marker.SequenceID]]
}

func NewMarkerManager() *MarkerManager {
	return &MarkerManager{
		sequenceManager: marker.NewSequenceManager(),
		lastUsedMap:     memstorage.New[marker.SequenceID, epoch.Index](),
		pruningMap:      memstorage.New[epoch.Index, set.Set[marker.SequenceID]](),
	}
}

// ProcessBlock returns the structure Details of a Block that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkerManager) ProcessBlock(block *Block, structureDetails []*marker.StructureDetails) (newStructureDetails *marker.StructureDetails, newSequenceCreated bool) {
	newStructureDetails, newSequenceCreated = m.sequenceManager.InheritStructureDetails(structureDetails)

	// TODO register marker -> block mapping
	// if newStructureDetails.IsPastMarker() {
	// 	m.SetBlockID(newStructureDetails.PastMarkers().Marker(), block.ID())
	// }

	// TODO: register in pruning map and lastUsedMap

	return
}
