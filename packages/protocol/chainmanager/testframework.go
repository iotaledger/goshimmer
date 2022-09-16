package chainmanager

import (
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type TestFramework struct {
	Manager *Manager

	test               *testing.T
	commitmentsByAlias map[string]*Commitment

	sync.RWMutex
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	return options.Apply(&TestFramework{
		Manager: NewManager(0, commitment.RootsID{0}, commitment.ID{0}),

		test:               test,
		commitmentsByAlias: make(map[string]*Commitment),
	}, opts, func(t *TestFramework) {
		t.commitmentsByAlias["Genesis"] = t.Manager.SnapshotCommitment
	})
}

func (t *TestFramework) CreateCommitment(alias string, prevAlias string) {
	t.Lock()
	defer t.Unlock()

	prevCommitmentID, previousIndex := t.previousCommitmentID(prevAlias)
	randomECR := blake2b.Sum256([]byte(alias + prevAlias))

	commitment := NewCommitment(commitment.NewID(previousIndex+1, randomECR, prevCommitmentID))
	commitment.PublishData(previousIndex+1, randomECR, prevCommitmentID)

	t.commitmentsByAlias[alias] = commitment
}

func (t *TestFramework) ProcessCommitment(alias string) (chain *Chain, wasForked bool) {
	commitment := t.commitment(alias)

	return t.Manager.ProcessCommitment(commitment.Index(), commitment.RootsID(), commitment.PrevID())
}

func (t *TestFramework) Chain(alias string) (chain *Chain) {
	return t.Manager.Chain(t.EC(alias))
}

func (t *TestFramework) commitment(alias string) (commitment *Commitment) {
	t.RLock()
	defer t.RUnlock()

	commitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the commitment does not exist")
	}

	return
}

func (t *TestFramework) EC(alias string) (epochCommitment commitment.ID) {
	return t.commitment(alias).ID
}

func (t *TestFramework) EI(alias string) (index epoch.Index) {
	return t.commitment(alias).Index()
}

func (t *TestFramework) ECR(alias string) (ecr commitment.RootsID) {
	return t.commitment(alias).RootsID()
}

func (t *TestFramework) PrevEC(alias string) (prevEC commitment.ID) {
	return t.commitment(alias).PrevID()
}

func (t *TestFramework) AssertChainIsAlias(chain *Chain, alias string) {
	if alias == "" {
		require.Nil(t.test, chain)
		return
	}

	require.Equal(t.test, t.commitment(alias).ID, chain.ForkingPoint.ID)
}

func (t *TestFramework) AssertChainState(chains map[string]string) {
	commitmentsByChainAlias := make(map[string][]string)

	for commitmentAlias, chainAlias := range chains {
		if chainAlias == "" {
			require.Nil(t.test, t.Chain(commitmentAlias))
			continue
		}
		commitmentsByChainAlias[chainAlias] = append(commitmentsByChainAlias[chainAlias], commitmentAlias)

		chain := t.Chain(commitmentAlias)

		require.NotNil(t.test, chain)
		require.Equal(t.test, t.EC(chainAlias), chain.ForkingPoint.ID)

	}

	for chainAlias, commitmentAliases := range commitmentsByChainAlias {
		chain := t.Chain(chainAlias)
		require.Equal(t.test, len(commitmentAliases), chain.Size())

		for _, commitmentAlias := range commitmentAliases {
			chainCommitment := chain.Commitment(t.EI(commitmentAlias))

			require.NotNil(t.test, chainCommitment)
			require.EqualValues(t.test, t.EC(commitmentAlias), chainCommitment.ID)
			require.EqualValues(t.test, t.EI(commitmentAlias), chainCommitment.Index())
			require.EqualValues(t.test, t.ECR(commitmentAlias), chainCommitment.RootsID())
			require.EqualValues(t.test, t.PrevEC(commitmentAlias), chainCommitment.PrevID())
		}
	}
}

func (t *TestFramework) previousCommitmentID(alias string) (previousCommitmentID commitment.ID, previousIndex epoch.Index) {
	if alias == "" {
		return
	}

	previousCommitment, exists := t.commitmentsByAlias[alias]
	if !exists {
		panic("the previous commitment does not exist")
	}

	return previousCommitment.ID, previousCommitment.Index()
}
