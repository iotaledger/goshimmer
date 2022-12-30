package virtualvoting

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type TestFramework struct {
	VirtualVoting *VirtualVoting

	test              *testing.T
	identitiesByAlias map[string]*identity.Identity
	trackedBlocks     uint32

	optsBlockDAG        *blockdag.BlockDAG
	optsBlockDAGOptions []options.Option[blockdag.BlockDAG]
	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsBooker          *booker.Booker
	optsBookerOptions   []options.Option[booker.Booker]
	optsVirtualVoting   []options.Option[VirtualVoting]
	optsValidators      *sybilprotection.WeightedSet

	*BookerTestFramework
	*VotesTestFramework
	*ConflictTrackerTestFramework
	*SequenceTrackerTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:              test,
		identitiesByAlias: make(map[string]*identity.Identity),
	}, opts, func(t *TestFramework) {
		t.BookerTestFramework = booker.NewTestFramework(
			test,
			lo.Cond(t.optsBlockDAG != nil, booker.WithBlockDAG(t.optsBlockDAG), booker.WithBlockDAGOptions(t.optsBlockDAGOptions...)),
			lo.Cond(t.optsLedger != nil, booker.WithLedger(t.optsLedger), booker.WithLedgerOptions(t.optsLedgerOptions...)),
			lo.Cond(t.optsBooker != nil, booker.WithBooker(t.optsBooker), booker.WithBookerOptions(t.optsBookerOptions...)),
		)

		if t.optsValidators == nil {
			t.optsValidators = sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))
		}

		if t.VirtualVoting == nil {
			t.VirtualVoting = New(t.Booker, t.optsValidators, t.optsVirtualVoting...)
		}

		t.VotesTestFramework = votes.NewTestFramework(test, votes.WithValidators(t.optsValidators))
		t.ConflictTrackerTestFramework = conflicttracker.NewTestFramework[BlockVotePower](test,
			conflicttracker.WithVotesTestFramework[BlockVotePower](t.VotesTestFramework),
			conflicttracker.WithConflictTracker(t.VirtualVoting.conflictTracker),
			conflicttracker.WithConflictDAG[BlockVotePower](t.VirtualVoting.Booker.Ledger.ConflictDAG),
			conflicttracker.WithValidators[BlockVotePower](t.optsValidators),
		)
		t.SequenceTrackerTestFramework = sequencetracker.NewTestFramework[BlockVotePower](test,
			sequencetracker.WithVotesTestFramework[BlockVotePower](t.VotesTestFramework),
			sequencetracker.WithSequenceTracker[BlockVotePower](t.VirtualVoting.sequenceTracker),
			sequencetracker.WithSequenceManager[BlockVotePower](t.BookerTestFramework.SequenceManager()),
			sequencetracker.WithValidators[BlockVotePower](t.optsValidators),
		)
	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.VirtualVoting.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) CreateIdentity(alias string, weight int64) {
	t.identitiesByAlias[alias] = identity.GenerateIdentity()
	t.CreateValidatorWithID(alias, t.identitiesByAlias[alias].ID(), weight)
}

func (t *TestFramework) Identity(alias string) (v *identity.Identity) {
	v, ok := t.identitiesByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) Identities(aliases ...string) (identities *set.AdvancedSet[*identity.Identity]) {
	identities = set.NewAdvancedSet[*identity.Identity]()
	for _, alias := range aliases {
		identities.Add(t.Identity(alias))
	}

	return
}

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*set.AdvancedSet[identity.ID]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.SequenceTracker.Voters(marker)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*set.AdvancedSet[identity.ID]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker.Voters(conflictID)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.test, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}

func (t *TestFramework) setupEvents() {
	t.VirtualVoting.Events.BlockTracked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.trackedBlocks), 1)
	}))
}

type BookerTestFramework = booker.TestFramework

type VotesTestFramework = votes.TestFramework

type ConflictTrackerTestFramework = conflicttracker.TestFramework[BlockVotePower]

type SequenceTrackerTestFramework = sequencetracker.TestFramework[BlockVotePower]

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockDAGOptions(opts ...options.Option[blockdag.BlockDAG]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAGOptions = opts
	}
}

func WithBlockDAG(blockDAG *blockdag.BlockDAG) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAG = blockDAG
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedgerOptions = opts
	}
}

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedger = ledger
	}
}

func WithBookerOptions(opts ...options.Option[booker.Booker]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsBookerOptions = opts
	}
}

func WithBooker(booker *booker.Booker) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBooker = booker
	}
}

func WithVirtualVotingOptions(opts ...options.Option[VirtualVoting]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsVirtualVoting = opts
	}
}

func WithVirtualVoting(virtualVoting *VirtualVoting) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.VirtualVoting = virtualVoting
	}
}

func WithValidators(validators *sybilprotection.WeightedSet) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidators = validators
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
