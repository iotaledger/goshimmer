package virtualvoting

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type TestFramework struct {
	Instance *VirtualVoting

	test              *testing.T
	identitiesByAlias map[string]*identity.Identity
	trackedBlocks     uint32

	Booker          *booker.TestFramework
	Ledger          *ledger.TestFramework
	BlockDAG        *blockdag.TestFramework
	Votes           *votes.TestFramework
	ConflictDAG     *conflictdag.TestFramework
	ConflictTracker *conflicttracker.TestFramework[BlockVotePower]
	SequenceTracker *sequencetracker.TestFramework[BlockVotePower]
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, virtualVotingInstance *VirtualVoting) *TestFramework {
	t := &TestFramework{
		test:              test,
		Instance:          virtualVotingInstance,
		identitiesByAlias: make(map[string]*identity.Identity),
	}

	t.Booker = booker.NewTestFramework(
		test,
		workers.CreateGroup("BookerTestFramework"),
		virtualVotingInstance.Booker,
	)

	t.Ledger = t.Booker.Ledger
	t.BlockDAG = t.Booker.BlockDAG

	t.Votes = votes.NewTestFramework(test, virtualVotingInstance.Validators)

	t.ConflictDAG = conflictdag.NewTestFramework(t.test, t.Ledger.Instance.ConflictDAG)

	t.ConflictTracker = conflicttracker.NewTestFramework[BlockVotePower](test,
		t.Votes,
		t.ConflictDAG,
		virtualVotingInstance.conflictTracker,
	)

	t.SequenceTracker = sequencetracker.NewTestFramework[BlockVotePower](test,
		t.Votes,
		virtualVotingInstance.sequenceTracker,
		t.Booker.SequenceManager(),
	)
	t.setupEvents()
	return t
}

func (t *TestFramework) ValidatorsSet(aliases ...string) (validators *set.AdvancedSet[identity.ID]) {
	return t.Votes.ValidatorsSet(aliases...)
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Instance.Block(t.Booker.BlockDAG.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) CreateIdentity(alias string, weight int64, skipWeightUpdate ...bool) {
	t.identitiesByAlias[alias] = identity.GenerateIdentity()
	identity.RegisterIDAlias(t.identitiesByAlias[alias].ID(), alias)
	t.Votes.CreateValidatorWithID(alias, t.identitiesByAlias[alias].ID(), weight, skipWeightUpdate...)
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

func (t *TestFramework) ValidatorsWithWeights(aliases ...string) map[identity.ID]uint64 {
	weights := make(map[identity.ID]uint64)

	for _, alias := range aliases {
		id := t.Identity(alias).ID()
		w, exists := t.Votes.Validators.Weights.Get(id)
		if exists {
			weights[id] = uint64(w.Value)
		}
	}

	return weights
}

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*set.AdvancedSet[identity.ID]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.SequenceTracker.Instance.Voters(marker)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*set.AdvancedSet[identity.ID]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker.Instance.Voters(conflictID)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.test, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}

func (t *TestFramework) setupEvents() {
	event.Hook(t.Instance.Events.BlockTracked, func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.trackedBlocks), 1)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
