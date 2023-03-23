package chainmanager

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	Instance *Manager

	test               *testing.T
	commitmentsByAlias map[string]*commitment.Commitment

	forkDetected              int32
	commitmentMissing         int32
	missingCommitmentReceived int32
	commitmentBelowRoot       int32

	sync.RWMutex
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	snapshotCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)

	return options.Apply(&TestFramework{
		Instance: NewManager(),

		test: test,
		commitmentsByAlias: map[string]*commitment.Commitment{
			"Genesis": snapshotCommitment,
		},
	}, opts, func(t *TestFramework) {
		t.Instance.Initialize(snapshotCommitment)
		t.Instance.Events.ForkDetected.Hook(func(fork *Fork) {
			t.test.Logf("ForkDetected: %s", fork)
			atomic.AddInt32(&t.forkDetected, 1)
		})
		t.Instance.Events.CommitmentMissing.Hook(func(id commitment.ID) {
			t.test.Logf("CommitmentMissing: %s", id)
			atomic.AddInt32(&t.commitmentMissing, 1)
		})
		t.Instance.Events.MissingCommitmentReceived.Hook(func(id commitment.ID) {
			t.test.Logf("MissingCommitmentReceived: %s", id)
			atomic.AddInt32(&t.missingCommitmentReceived, 1)
		})
		t.Instance.Events.CommitmentBelowRoot.Hook(func(id commitment.ID) {
			t.test.Logf("CommitmentBelowRoot: %s", id)
			atomic.AddInt32(&t.commitmentBelowRoot, 1)
		})
	})
}

func (t *TestFramework) CreateCommitment(alias string, prevAlias string) {
	t.Lock()
	defer t.Unlock()

	prevCommitmentID, previousIndex := t.previousCommitmentID(prevAlias)
	randomECR := blake2b.Sum256([]byte(alias + prevAlias))

	t.commitmentsByAlias[alias] = commitment.New(previousIndex+1, prevCommitmentID, randomECR, 0)
	t.commitmentsByAlias[alias].ID().RegisterAlias(alias)
}

func (t *TestFramework) ProcessCommitment(alias string) (isSolid bool, chain *Chain) {
	return t.Instance.ProcessCommitment(t.commitment(alias))
}

func (t *TestFramework) ProcessCommitmentFromOtherSource(alias string) (isSolid bool, chain *Chain) {
	return t.Instance.ProcessCommitmentFromSource(t.commitment(alias), identity.NewID(ed25519.PublicKey{}))
}

func (t *TestFramework) Chain(alias string) (chain *Chain) {
	return t.Instance.Chain(t.SlotCommitment(alias))
}

func (t *TestFramework) commitment(alias string) (commitment *commitment.Commitment) {
	t.RLock()
	defer t.RUnlock()

	commitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the commitment does not exist")
	}

	return
}

func (t *TestFramework) ChainCommitment(alias string) *Commitment {
	cm, exists := t.Instance.commitment(t.SlotCommitment(alias))
	require.True(t.test, exists)

	return cm
}

func (t *TestFramework) AssertForkDetectedCount(expected int) {
	require.EqualValues(t.test, expected, t.forkDetected, "forkDetected count does not match")
}

func (t *TestFramework) AssertCommitmentMissingCount(expected int) {
	require.EqualValues(t.test, expected, t.commitmentMissing, "commitmentMissing count does not match")
}

func (t *TestFramework) AssertMissingCommitmentReceivedCount(expected int) {
	require.EqualValues(t.test, expected, t.missingCommitmentReceived, "missingCommitmentReceived count does not match")
}

func (t *TestFramework) AssertCommitmentBelowRootCount(expected int) {
	require.EqualValues(t.test, expected, t.commitmentBelowRoot, "commitmentBelowRoot count does not match")
}

func (t *TestFramework) AssertEqualChainCommitments(commitments []*Commitment, aliases ...string) {
	var chainCommitments []*Commitment
	for _, alias := range aliases {
		chainCommitments = append(chainCommitments, t.ChainCommitment(alias))
	}

	require.EqualValues(t.test, commitments, chainCommitments)
}

func (t *TestFramework) SlotCommitment(alias string) commitment.ID {
	return t.commitment(alias).ID()
}

func (t *TestFramework) SlotIndex(alias string) slot.Index {
	return t.commitment(alias).Index()
}

func (t *TestFramework) SlotCommitmentRoot(alias string) types.Identifier {
	return t.commitment(alias).RootsID()
}

func (t *TestFramework) PrevSlotCommitment(alias string) commitment.ID {
	return t.commitment(alias).PrevID()
}

func (t *TestFramework) AssertChainIsAlias(chain *Chain, alias string) {
	if alias == "" {
		require.Nil(t.test, chain)
		return
	}

	require.Equal(t.test, t.commitment(alias).ID(), chain.ForkingPoint.ID())
}

func (t *TestFramework) AssertChainState(chains map[string]string) {
	commitmentsByChainAlias := make(map[string][]string)

	for commitmentAlias, chainAlias := range chains {
		if chainAlias == "" {
			require.Nil(t.test, t.Chain(commitmentAlias))
			continue
		}
		if chainAlias == "evicted" {
			_, exists := t.Instance.commitment(t.SlotCommitment(commitmentAlias))
			require.False(t.test, exists, "commitment %s should be pruned", commitmentAlias)
			continue
		}
		commitmentsByChainAlias[chainAlias] = append(commitmentsByChainAlias[chainAlias], commitmentAlias)

		chain := t.Chain(commitmentAlias)

		require.NotNil(t.test, chain, "chain for commitment %s is nil", commitmentAlias)
		require.Equal(t.test, t.SlotCommitment(chainAlias), chain.ForkingPoint.ID())
	}
}

func (t *TestFramework) previousCommitmentID(alias string) (previousCommitmentID commitment.ID, previousIndex slot.Index) {
	if alias == "" {
		return
	}

	previousCommitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the previous commitment does not exist")
	}

	return previousCommitment.ID(), previousCommitment.Index()
}
