package blockgadget

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/module"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test          *testing.T
	Gadget        Gadget
	Tangle        *tangle.TestFramework
	VirtualVoting *booker.VirtualVotingTestFramework
	MemPool       *mempool.TestFramework
	BlockDAG      *blockdag.TestFramework
	Votes         *votes.TestFramework

	acceptedBlocks    uint32
	confirmedBlocks   uint32
	conflictsAccepted uint32
	conflictsRejected uint32
}

func NewTestFramework(test *testing.T, gadget Gadget, tangleTF *tangle.TestFramework) *TestFramework {
	t := &TestFramework{
		test:          test,
		Gadget:        gadget,
		Tangle:        tangleTF,
		VirtualVoting: tangleTF.VirtualVoting,
		MemPool:       tangleTF.MemPool,
		BlockDAG:      tangleTF.BlockDAG,
		Votes:         tangleTF.Votes,
	}

	t.setupEvents()
	return t
}

func (t *TestFramework) setupEvents() {
	t.Gadget.Events().BlockAccepted.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("ACCEPTED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.acceptedBlocks), 1)
	})

	t.Gadget.Events().BlockConfirmed.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("CONFIRMED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.confirmedBlocks), 1)
	})

	t.Tangle.VirtualVoting.ConflictDAG.Instance.Events.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT ACCEPTED: %s", conflict.ID())
		}
		atomic.AddUint32(&(t.conflictsAccepted), 1)
	})

	t.Tangle.VirtualVoting.ConflictDAG.Instance.Events.ConflictRejected.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT REJECTED: %s", conflict.ID())
		}

		atomic.AddUint32(&(t.conflictsRejected), 1)
	})
}

func (t *TestFramework) AssertBlockAccepted(blocksAccepted uint32) {
	require.Equal(t.test, blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks), "expected %d blocks to be accepted but got %d", blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) AssertBlockConfirmed(blocksConfirmed uint32) {
	require.Equal(t.test, blocksConfirmed, atomic.LoadUint32(&t.confirmedBlocks), "expected %d blocks to be accepted but got %d", blocksConfirmed, atomic.LoadUint32(&t.confirmedBlocks))
}

func (t *TestFramework) AssertConflictsAccepted(conflictsAccepted uint32) {
	require.Equal(t.test, conflictsAccepted, atomic.LoadUint32(&t.conflictsAccepted), "expected %d conflicts to be accepted but got %d", conflictsAccepted, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) AssertConflictsRejected(conflictsRejected uint32) {
	require.Equal(t.test, conflictsRejected, atomic.LoadUint32(&t.conflictsRejected), "expected %d conflicts to be rejected but got %d", conflictsRejected, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) ValidateAcceptedBlocks(expectedAcceptedBlocks map[string]bool) {
	for blockID, blockExpectedAccepted := range expectedAcceptedBlocks {
		actualBlockAccepted := t.Gadget.IsBlockAccepted(t.Tangle.BlockDAG.Block(blockID).ID())
		require.Equal(t.test, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (t *TestFramework) ValidateConfirmedBlocks(expectedConfirmedBlocks map[string]bool) {
	for blockID, blockExpectedConfirmed := range expectedConfirmedBlocks {
		actualBlockConfirmed := t.Gadget.IsBlockConfirmed(t.Tangle.BlockDAG.Block(blockID).ID())
		require.Equal(t.test, blockExpectedConfirmed, actualBlockConfirmed, "Block %s should be confirmed=%t but is %t", blockID, blockExpectedConfirmed, actualBlockConfirmed)
	}
}

func (t *TestFramework) ValidateAcceptedMarker(expectedConflictIDs map[markers.Marker]bool) {
	for marker, markerExpectedAccepted := range expectedConflictIDs {
		actualMarkerAccepted := t.Gadget.IsMarkerAccepted(marker)
		require.Equal(t.test, markerExpectedAccepted, actualMarkerAccepted, "%s should be accepted=%t but is %t", marker, markerExpectedAccepted, actualMarkerAccepted)
	}
}

func (t *TestFramework) ValidateConflictAcceptance(expectedConflictIDs map[string]confirmation.State) {
	for conflictIDAlias, conflictExpectedState := range expectedConflictIDs {
		actualMarkerAccepted := t.Tangle.VirtualVoting.ConflictDAG.Instance.ConfirmationState(advancedset.New(t.Tangle.MemPool.Transaction(conflictIDAlias).ID()))
		require.Equal(t.test, conflictExpectedState, actualMarkerAccepted, "%s should be accepted=%s but is %s", conflictIDAlias, conflictExpectedState, actualMarkerAccepted)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// MockBlockGadget mocks a block gadget marking all blocks as confirmed.
type MockBlockGadget struct {
	events          *Events
	AcceptedBlocks  models.BlockIDs
	AcceptedMarkers *markers.Markers

	mutex sync.RWMutex

	module.Module
}

var _ Gadget = new(MockBlockGadget)

func NewMockAcceptanceGadget() *MockBlockGadget {
	g := &MockBlockGadget{
		events:          NewEvents(),
		AcceptedBlocks:  models.NewBlockIDs(),
		AcceptedMarkers: markers.NewMarkers(),
	}
	g.TriggerConstructed()
	g.TriggerInitialized()
	return g
}

func (m *MockBlockGadget) Events() *Events {
	return m.events
}

func (m *MockBlockGadget) IsBlockConfirmed(blockID models.BlockID) bool {
	// If a block is accepted, then it is automatically confirmed
	return m.IsBlockAccepted(blockID)
}

func (m *MockBlockGadget) SetBlockAccepted(block *Block) {
	m.mutex.Lock()
	m.AcceptedBlocks.Add(block.ID())
	m.mutex.Unlock()

	m.events.BlockAccepted.Trigger(block)
}

func (m *MockBlockGadget) SetMarkersAccepted(markers ...markers.Marker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, marker := range markers {
		m.AcceptedMarkers.Set(marker.SequenceID(), marker.Index())
	}
}

// IsBlockAccepted mocks its interface function returning that all blocks are confirmed.
func (m *MockBlockGadget) IsBlockAccepted(blockID models.BlockID) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.AcceptedBlocks.Contains(blockID)
}

func (m *MockBlockGadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if marker.Index() == 0 {
		return true
	}

	if m.AcceptedMarkers == nil || m.AcceptedMarkers.Size() == 0 {
		return false
	}
	acceptedIndex, exists := m.AcceptedMarkers.Get(marker.SequenceID())
	if !exists {
		return false
	}
	return marker.Index() <= acceptedIndex
}

func (m *MockBlockGadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	acceptedIndex, exists := m.AcceptedMarkers.Get(sequenceID)
	if exists {
		return acceptedIndex + 1
	}
	return 1
}

func (m *MockBlockGadget) AcceptedBlocksInSlot(index slot.Index) (blocks models.BlockIDs) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	blocks = models.NewBlockIDs()
	for _, block := range m.AcceptedBlocks.Slice() {
		if block.Index() == index {
			blocks.Add(block)
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
