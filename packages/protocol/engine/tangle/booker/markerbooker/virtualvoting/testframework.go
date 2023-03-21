package virtualvoting

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/debug"
)

type TestFramework struct {
	Instance *VirtualVoting

	test              *testing.T
	identitiesByAlias map[string]*identity.Identity
	trackedBlocks     uint32

	ConflictDAG     *conflictdag.TestFramework
	Votes           *votes.TestFramework
	ConflictTracker *conflicttracker.TestFramework[BlockVotePower]
	SequenceTracker *sequencetracker.TestFramework[BlockVotePower]
}

func NewTestFramework(test *testing.T, virtualVotingInstance *VirtualVoting) *TestFramework {
	t := &TestFramework{
		test:              test,
		Instance:          virtualVotingInstance,
		identitiesByAlias: make(map[string]*identity.Identity),
	}

	t.ConflictDAG = conflictdag.NewTestFramework(t.test, virtualVotingInstance.ConflictDAG)

	t.Votes = votes.NewTestFramework(test, virtualVotingInstance.Validators)

	t.ConflictTracker = conflicttracker.NewTestFramework(test,
		t.Votes,
		t.ConflictDAG,
		virtualVotingInstance.conflictTracker,
	)

	t.SequenceTracker = sequencetracker.NewTestFramework(test,
		t.Votes,
		virtualVotingInstance.sequenceTracker,
		virtualVotingInstance.SequenceManager,
	)
	t.setupEvents()
	return t
}

func (t *TestFramework) ValidatorsSet(aliases ...string) (validators *advancedset.AdvancedSet[identity.ID]) {
	return t.Votes.ValidatorsSet(aliases...)
}

func (t *TestFramework) RegisterIdentity(alias string, id *identity.Identity) {
	t.identitiesByAlias[alias] = id
	identity.RegisterIDAlias(t.identitiesByAlias[alias].ID(), alias)
}

func (t *TestFramework) CreateIdentity(alias string, weight int64, skipWeightUpdate ...bool) {
	t.RegisterIdentity(alias, identity.GenerateIdentity())
	t.Votes.CreateValidatorWithID(alias, t.identitiesByAlias[alias].ID(), weight, skipWeightUpdate...)
}

func (t *TestFramework) Identity(alias string) (v *identity.Identity) {
	v, ok := t.identitiesByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) Identities(aliases ...string) (identities *advancedset.AdvancedSet[*identity.Identity]) {
	identities = advancedset.New[*identity.Identity]()
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

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*advancedset.AdvancedSet[identity.ID]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.SequenceTracker.Instance.Voters(marker)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) ValidateConflictVoters(expectedVoters map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]) {
	for conflictID, expectedVotersOfMarker := range expectedVoters {
		voters := t.ConflictTracker.Instance.Voters(conflictID)

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "conflict %s expected %d voters but got %d", conflictID, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.test, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}

func (t *TestFramework) setupEvents() {
	t.Instance.Events().BlockTracked.Hook(func(metadata *booker.Block) {
		if debug.GetEnabled() {
			t.test.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.trackedBlocks), 1)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
