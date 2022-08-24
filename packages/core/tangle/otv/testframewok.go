package otv

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type TestFramework struct {
	ValidatorSet *validator.Set
	OTV          *OnTangleVoting

	test              *testing.T
	identitiesByAlias map[string]*identity.Identity
	trackedBlocks     uint32

	optsBlockDAG []options.Option[blockdag.BlockDAG]
	optsBooker   []options.Option[booker.Booker]
	optsOTV      []options.Option[OnTangleVoting]

	*BookerTestFramework
	*VotesTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:              test,
		identitiesByAlias: make(map[string]*identity.Identity),
	}, opts, func(t *TestFramework) {
		t.ValidatorSet = validator.NewSet()

		t.BookerTestFramework = booker.NewTestFramework(test, booker.WithBlockDAGOptions(t.optsBlockDAG...), booker.WithBookerOptions(t.optsBooker...))
		t.OTV = New(t.EvictionManager, t.Ledger(), t.BlockDAG, t.Booker, t.ValidatorSet, t.optsOTV...)
		t.VotesTestFramework = votes.NewTestFramework[BlockVotePower](
			test,
			votes.WithValidatorSet[BlockVotePower](t.ValidatorSet),
			votes.WithConflictTracker(t.OTV.conflictTracker),
			votes.WithSequenceTracker(t.OTV.sequenceTracker),
			votes.WithConflictDAG[BlockVotePower](t.OTV.Booker.Ledger.ConflictDAG),
			votes.WithSequenceManager[BlockVotePower](t.BookerTestFramework.SequenceManager()),
		)
	}, (*TestFramework).setupEvents)
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
		voters := t.SequenceTracker().Voters(marker)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker().Voters(conflictID)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.test, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}

func (t *TestFramework) setupEvents() {
	t.OTV.Events.BlockTracked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.trackedBlocks), 1)
	}))
}

type BookerTestFramework = booker.TestFramework

type VotesTestFramework = votes.TestFramework[BlockVotePower]

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockDAGOptions(opts ...options.Option[blockdag.BlockDAG]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAG = opts
	}
}

func WithBookerOptions(opts ...options.Option[booker.Booker]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsBooker = opts
	}
}

func WithOnTangleVotingOptions(opts ...options.Option[OnTangleVoting]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsOTV = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
