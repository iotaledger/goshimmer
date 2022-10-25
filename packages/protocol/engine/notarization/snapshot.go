package notarization

import (
	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
)

// LoadOutputsWithMetadata initiates the state and mana trees from a given snapshot.
func (m *Manager) LoadOutputsWithMetadata(outputsWithMetadatas []*chainstorage.OutputWithMetadata) {
	m.advanceStateRoots(0, []*chainstorage.OutputWithMetadata{}, outputsWithMetadatas)
}

func (m *Manager) RollbackOutputs(outputsWithMetadata []*chainstorage.OutputWithMetadata, areCreated bool) {
	if areCreated {
		m.advanceStateRoots(0, outputsWithMetadata, []*chainstorage.OutputWithMetadata{})
	} else {
		m.advanceStateRoots(0, []*chainstorage.OutputWithMetadata{}, outputsWithMetadata)
	}
}
