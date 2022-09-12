package epoch

import (
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type TestFramework struct {
	Manager *CommitmentChainManager

	test               *testing.T
	commitmentsByAlias map[string]*Commitment

	sync.RWMutex
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	return options.Apply(&TestFramework{
		Manager:            NewCommitmentChainManager(0, epoch.EC{0}),
		test:               test,
		commitmentsByAlias: make(map[string]*Commitment),
	}, opts, func(t *TestFramework) {
		t.commitmentsByAlias["genesis"] = t.Manager.GenesisCommitment
	})
}

func (t *TestFramework) AddCommitment(alias string, prevAlias string) {
	t.Lock()
	defer t.Unlock()

	prevCommitmentID, previousIndex := t.previousCommitmentID(prevAlias)
	randomECR := blake2b.Sum256([]byte(alias + prevAlias))

	commitment := NewCommitment(NewCommitmentID(previousIndex+1, randomECR, prevCommitmentID))
	commitment.publishECRecord(previousIndex+1, randomECR, prevCommitmentID)

	t.commitmentsByAlias[alias] = commitment
}

func (t *TestFramework) Commitment(alias string) (commitment *Commitment) {
	t.RLock()
	defer t.RUnlock()

	commitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the commitment does not exist")
	}

	return commitment
}

func (t *TestFramework) Chain(alias string) (chain *CommitmentChain) {
	commitment := t.Commitment(alias)

	return t.Manager.Chain(commitment.EI(), commitment.ECR(), commitment.PrevEC())
}

func (t *TestFramework) AssertChain(chain *CommitmentChain, alias string) {
	if alias == "" {
		require.Nil(t.test, chain)
		return
	}

	require.Equal(t.test, t.Commitment(alias).ID, chain.ForkingPoint.ID)
}

func (t *TestFramework) AssertChains(chains map[string]string) {
	for commitmentAlias, chainAlias := range chains {
		if chainAlias == "" {
			require.Nil(t.test, t.Manager.Commitment(t.Commitment(commitmentAlias).ID).Chain())
			continue
		}

		chain := t.Manager.Commitment(t.Commitment(commitmentAlias).ID).Chain()

		require.NotNil(t.test, chain)
		require.Equal(t.test, t.Commitment(chainAlias).ID, chain.ForkingPoint.ID)
	}
}

func (t *TestFramework) previousCommitmentID(alias string) (previousCommitmentID epoch.EC, previousIndex epoch.Index) {
	if alias == "" {
		return
	}

	previousCommitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the previous commitment does not exist")
	}

	return previousCommitment.ID, previousCommitment.EI()
}
