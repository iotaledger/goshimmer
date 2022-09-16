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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/blockdag"
	booker2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/utxo"
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
	optsBooker          *booker2.Booker
	optsBookerOptions   []options.Option[booker2.Booker]
	optsVirtualVoting   []options.Option[VirtualVoting]

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
		t.BookerTestFramework = booker2.NewTestFramework(
			test,
			lo.Cond(t.optsBlockDAG != nil, booker2.WithBlockDAG(t.optsBlockDAG), booker2.WithBlockDAGOptions(t.optsBlockDAGOptions...)),
			lo.Cond(t.optsLedger != nil, booker2.WithLedger(t.optsLedger), booker2.WithLedgerOptions(t.optsLedgerOptions...)),
			lo.Cond(t.optsBooker != nil, booker2.WithBooker(t.optsBooker), booker2.WithBookerOptions(t.optsBookerOptions...)),
		)

		if t.VirtualVoting == nil {
			t.VirtualVoting = New(t.Booker, validator.NewSet(), t.optsVirtualVoting...)
		}

		t.VotesTestFramework = votes.NewTestFramework(test, votes.WithValidatorSet(t.VirtualVoting.ValidatorSet))
		t.ConflictTrackerTestFramework = conflicttracker.NewTestFramework[BlockVotePower](test,
			conflicttracker.WithVotesTestFramework[BlockVotePower](t.VotesTestFramework),
			conflicttracker.WithConflictTracker(t.VirtualVoting.conflictTracker),
			conflicttracker.WithConflictDAG[BlockVotePower](t.VirtualVoting.Booker.Ledger.ConflictDAG),
		)
		t.SequenceTrackerTestFramework = sequencetracker.NewTestFramework[BlockVotePower](test,
			sequencetracker.WithVotesTestFramework[BlockVotePower](t.VotesTestFramework),
			sequencetracker.WithSequenceTracker[BlockVotePower](t.VirtualVoting.sequenceTracker),
			sequencetracker.WithSequenceManager[BlockVotePower](t.BookerTestFramework.SequenceManager()),
		)
	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.VirtualVoting.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) CreateIdentity(alias string, opts ...options.Option[validator.Validator]) {
	t.identitiesByAlias[alias] = identity.GenerateIdentity()
	t.CreateValidatorWithID(alias, t.identitiesByAlias[alias].ID(), opts...)
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

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*set.AdvancedSet[*validator.Validator]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.SequenceTracker.Voters(marker)

		assert.True(t.test, expectedVotersOfMarker.Equal(votes.ValidatorSetToAdvancedSet(voters)), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker.Voters(conflictID)

		assert.True(t.test, expectedVotersOfMarker.Equal(votes.ValidatorSetToAdvancedSet(voters)), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
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

type BookerTestFramework = booker2.TestFramework

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

func WithBookerOptions(opts ...options.Option[booker2.Booker]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsBookerOptions = opts
	}
}

func WithBooker(booker *booker2.Booker) options.Option[TestFramework] {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
