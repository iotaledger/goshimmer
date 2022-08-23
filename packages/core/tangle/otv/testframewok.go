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

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type bookerTestFramework = *booker.TestFramework
type votesTestFramework = *votes.TestFramework[BlockVotePower]

type TestFramework struct {
	evictionManager   *eviction.Manager[models.BlockID]
	identitiesByAlias map[string]*identity.Identity
	otv               *OnTangleVoting
	trackedBlocks     uint32
	optsOTV           []options.Option[OnTangleVoting]

	t *testing.T

	votesTestFramework
	bookerTestFramework
}

func NewTestFramework(t *testing.T, opts ...options.Option[TestFramework]) (tf *TestFramework) {
	tf = options.Apply(&TestFramework{
		identitiesByAlias: make(map[string]*identity.Identity),
		t:                 t,
	}, opts)

	validatorSet := validator.NewSet()
	tf.otv = New(validatorSet, tf.EvictionManager(), tf.optsOTV...)
	tf.bookerTestFramework = booker.NewTestFramework(t, booker.WithBooker(tf.OTV().Booker), booker.WithEvictionManager(tf.EvictionManager()))
	tf.votesTestFramework = votes.NewTestFramework[BlockVotePower](t, votes.WithValidatorSet[BlockVotePower](validatorSet), votes.WithConflictTracker(tf.otv.conflictTracker), votes.WithSequenceTracker(tf.otv.sequenceTracker), votes.WithConflictDAG[BlockVotePower](tf.otv.Booker.Ledger.ConflictDAG), votes.WithSequenceManager[BlockVotePower](tf.bookerTestFramework.SequenceManager()))

	tf.OTV().Events.BlockTracked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			tf.T.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(tf.trackedBlocks), 1)
	}))

	return
}

func (t *TestFramework) OTV() *OnTangleVoting {
	return t.otv
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

func (t *TestFramework) EvictionManager() *eviction.Manager[models.BlockID] {
	if t.evictionManager == nil {
		if t.otv != nil {
			t.evictionManager = t.otv.evictionManager.Manager
		} else {
			t.evictionManager = eviction.NewManager(models.IsEmptyBlockID)
		}
	}

	return t.evictionManager
}

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*set.AdvancedSet[*validator.Validator]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.SequenceTracker().Voters(marker)

		assert.True(t.t, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker().Voters(conflictID)

		assert.True(t.t, expectedVotersOfMarker.Equal(voters), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.t, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithOnTangleVotingOptions(opts ...options.Option[OnTangleVoting]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.otv != nil {
			panic("OTV already set")
		}
		tf.optsOTV = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
