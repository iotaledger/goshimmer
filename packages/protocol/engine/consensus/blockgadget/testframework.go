package blockgadget

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Gadget *Gadget

	test              *testing.T
	acceptedBlocks    uint32
	confirmedBlocks   uint32
	conflictsAccepted uint32
	conflictsRejected uint32
	reorgCount        uint32

	optsGadgetOptions       []options.Option[Gadget]
	optsLedger              *ledger.Ledger
	optsLedgerOptions       []options.Option[ledger.Ledger]
	optsEvictionState       *eviction.State
	optsTangle              *tangle.Tangle
	optsTangleOptions       []options.Option[tangle.Tangle]
	optsTotalWeightCallback func() int64
	optsValidators          *sybilprotection.WeightedSet

	*TangleTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
		optsTotalWeightCallback: func() int64 {
			return t.TangleTestFramework.Validators.TotalWeight()
		},
	}, opts, func(t *TestFramework) {
		if t.optsValidators == nil {
			t.optsValidators = sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))
		}

		if t.Gadget == nil {
			if t.optsTangle == nil {
				storageInstance := storage.New(test.TempDir(), 1)
				test.Cleanup(func() {
					t.optsLedger.Shutdown()
					storageInstance.Shutdown()
				})

				if t.optsLedger == nil {
					t.optsLedger = ledger.New(storageInstance, t.optsLedgerOptions...)
				}

				if t.optsEvictionState == nil {
					t.optsEvictionState = eviction.NewState(storageInstance)
				}

				t.optsTangle = tangle.New(t.optsLedger, t.optsEvictionState, t.optsValidators, func() epoch.Index {
					return 0
				}, func(id markers.SequenceID) markers.Index {
					return 1
				}, t.optsTangleOptions...)
			}

			t.Gadget = New(t.optsTangle, t.optsEvictionState, t.optsTotalWeightCallback, t.optsGadgetOptions...)
		}

		if t.TangleTestFramework == nil {
			t.TangleTestFramework = tangle.NewTestFramework(test, tangle.WithTangle(t.optsTangle), tangle.WithValidators(t.optsValidators))
		}
	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) setupEvents() {
	t.Gadget.Events.BlockAccepted.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("ACCEPTED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.acceptedBlocks), 1)
	}))

	t.Gadget.Events.BlockConfirmed.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("CONFIRMED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.confirmedBlocks), 1)
	}))

	t.Gadget.Events.Reorg.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		if debug.GetEnabled() {
			t.test.Logf("REORG NEEDED: %s", conflictID)
		}
		atomic.AddUint32(&(t.reorgCount), 1)
	}))

	t.ConflictDAG().Events.ConflictAccepted.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT ACCEPTED: %s", conflictID)
		}
		atomic.AddUint32(&(t.conflictsAccepted), 1)
	}))

	t.ConflictDAG().Events.ConflictRejected.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT REJECTED: %s", conflictID)
		}

		atomic.AddUint32(&(t.conflictsRejected), 1)
	}))
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

func (t *TestFramework) AssertReorgs(reorgCount uint32) {
	require.Equal(t.test, reorgCount, atomic.LoadUint32(&t.reorgCount), "expected %d reorgs but got %d", reorgCount, atomic.LoadUint32(&t.reorgCount))
}

func (t *TestFramework) ValidateAcceptedBlocks(expectedAcceptedBlocks map[string]bool) {
	for blockID, blockExpectedAccepted := range expectedAcceptedBlocks {
		actualBlockAccepted := t.Gadget.IsBlockAccepted(t.Block(blockID).ID())
		require.Equal(t.test, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (t *TestFramework) ValidateConfirmedBlocks(expectedConfirmedBlocks map[string]bool) {
	for blockID, blockExpectedConfirmed := range expectedConfirmedBlocks {
		actualBlockConfirmed := t.Gadget.isBlockConfirmed(t.Block(blockID).ID())
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
		actualMarkerAccepted := t.ConflictDAG().ConfirmationState(set.NewAdvancedSet(t.Transaction(conflictIDAlias).ID()))
		require.Equal(t.test, conflictExpectedState, actualMarkerAccepted, "%s should be accepted=%t but is %t", conflictIDAlias, conflictExpectedState, actualMarkerAccepted)
	}
}

type TangleTestFramework = tangle.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithGadget(gadget *Gadget) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.Gadget = gadget
	}
}

func WithTotalWeightCallback(totalWeightCallback func() int64) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTotalWeightCallback = totalWeightCallback
	}
}

func WithGadgetOptions(opts ...options.Option[Gadget]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsGadgetOptions = opts
	}
}

func WithTangle(tangle *tangle.Tangle) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = tangle
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangleOptions = opts
	}
}

func WithTangleTestFramework(testFramework *tangle.TestFramework) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.TangleTestFramework = testFramework
	}
}

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsLedger = ledger
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsLedgerOptions = opts
	}
}

func WithEvictionState(evictionState *eviction.State) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionState = evictionState
	}
}

func WithValidators(validators *sybilprotection.WeightedSet) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidators = validators
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// MockAcceptanceGadget mocks ConfirmationOracle marking all blocks as confirmed.
type MockAcceptanceGadget struct {
	BlockAcceptedEvent *event.Linkable[*Block]
	AcceptedBlocks     models.BlockIDs
	AcceptedMarkers    *markers.Markers

	mutex sync.RWMutex
}

func NewMockAcceptanceGadget() *MockAcceptanceGadget {
	return &MockAcceptanceGadget{
		BlockAcceptedEvent: event.NewLinkable[*Block](),
		AcceptedBlocks:     models.NewBlockIDs(),
		AcceptedMarkers:    markers.NewMarkers(),
	}
}

func (m *MockAcceptanceGadget) SetBlockAccepted(block *Block) {
	m.mutex.Lock()
	m.AcceptedBlocks.Add(block.ID())
	m.mutex.Unlock()

	m.BlockAcceptedEvent.Trigger(block)
}

func (m *MockAcceptanceGadget) SetMarkersAccepted(markers ...markers.Marker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, marker := range markers {
		m.AcceptedMarkers.Set(marker.SequenceID(), marker.Index())
	}
}

// IsBlockAccepted mocks its interface function returning that all blocks are confirmed.
func (m *MockAcceptanceGadget) IsBlockAccepted(blockID models.BlockID) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.AcceptedBlocks.Contains(blockID)
}

func (m *MockAcceptanceGadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
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

func (m *MockAcceptanceGadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	acceptedIndex, exists := m.AcceptedMarkers.Get(sequenceID)
	if exists {
		return acceptedIndex + 1
	}
	return 1
}

func (m *MockAcceptanceGadget) AcceptedBlocksInEpoch(index epoch.Index) (blocks models.BlockIDs) {
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
