package marker

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type SequenceID uint64

type Marker struct {
	sequenceID SequenceID
	index      uint64
}

type Sequence struct {
	id              SequenceID
	highestIndex    uint64
	parents         []Marker
	defaultBranchID ledgerstate.BranchID
}

type Manager struct {
	lastSequenceID    uint64
	lastSequenceMutex sync.RWMutex
}

func (m *Manager) NewSequence(branchID ledgerstate.BranchID, parents ...Marker) (sequence *Sequence) {
	m.lastSequenceMutex.Lock()
	defer m.lastSequenceMutex.Unlock()

	highestIndex := uint64(0)
	for _, parent := range parents {
		if parent.index > highestIndex {
			highestIndex = parent.index
		}
	}

	sequence = &Sequence{
		id:              SequenceID(m.lastSequenceID),
		highestIndex:    highestIndex + 1,
		parents:         parents,
		defaultBranchID: branchID,
	}

	m.lastSequenceID++

	return
}

type MarkerSth struct {
}

func (m *MarkerSth) BranchID(markerIndex uint64) (branchID ledgerstate.BranchID) {
	return
}
