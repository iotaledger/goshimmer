package chainmanager

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type TestFramework struct {
	Instance *Manager

	test               *testing.T
	commitmentsByAlias map[string]*commitment.Commitment

	sync.RWMutex
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	snapshotCommitment := commitment.New(0, commitment.ID{}, types.Identifier{}, 0)

	return options.Apply(&TestFramework{
		Instance: NewManager(snapshotCommitment),

		test: test,
		commitmentsByAlias: map[string]*commitment.Commitment{
			"Genesis": snapshotCommitment,
		},
	}, opts)
}

func (t *TestFramework) CreateCommitment(alias string, prevAlias string) {
	t.Lock()
	defer t.Unlock()

	prevCommitmentID, previousIndex := t.previousCommitmentID(prevAlias)
	randomECR := blake2b.Sum256([]byte(alias + prevAlias))

	t.commitmentsByAlias[alias] = commitment.New(previousIndex+1, prevCommitmentID, randomECR, 0)
}

func (t *TestFramework) ProcessCommitment(alias string) (isSolid bool, chain *Chain) {
	return t.Instance.ProcessCommitment(t.commitment(alias))
}

func (t *TestFramework) ProcessCommitmentFromOtherSource(alias string) (isSolid bool, chain *Chain) {
	return t.Instance.ProcessCommitmentFromSource(t.commitment(alias), identity.NewID(ed25519.PublicKey{}))
}

func (t *TestFramework) Chain(alias string) (chain *Chain) {
	return t.Instance.Chain(t.EC(alias))
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
	cm, created := t.Instance.Commitment(t.EC(alias))
	require.False(t.test, created)

	return cm
}

func (t *TestFramework) AssertEqualChainCommitments(commitments []*Commitment, aliases ...string) {
	var chainCommitments []*Commitment
	for _, alias := range aliases {
		chainCommitments = append(chainCommitments, t.ChainCommitment(alias))
	}

	require.EqualValues(t.test, commitments, chainCommitments)
}

func (t *TestFramework) EC(alias string) (epochCommitment commitment.ID) {
	return t.commitment(alias).ID()
}

func (t *TestFramework) EI(alias string) (index epoch.Index) {
	return t.commitment(alias).Index()
}

func (t *TestFramework) ECR(alias string) (ecr types.Identifier) {
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

	require.Equal(t.test, t.commitment(alias).ID(), chain.ForkingPoint.ID())
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
		require.Equal(t.test, t.EC(chainAlias), chain.ForkingPoint.ID())
	}

	for chainAlias, commitmentAliases := range commitmentsByChainAlias {
		chain := t.Chain(chainAlias)
		require.Equal(t.test, len(commitmentAliases), chain.Size())

		for _, commitmentAlias := range commitmentAliases {
			chainCommitment := chain.Commitment(t.EI(commitmentAlias))

			require.NotNil(t.test, chainCommitment)
			require.EqualValues(t.test, t.EC(commitmentAlias), chainCommitment.ID())
			require.EqualValues(t.test, t.commitment(commitmentAlias), chainCommitment.Commitment())
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

	return previousCommitment.ID(), previousCommitment.Index()
}
