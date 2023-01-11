package notarization

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type TestFramework struct {
	MutationFactory *EpochMutations

	test               *testing.T
	transactionsByID   map[string]*ledger.TransactionMetadata
	issuersByID        map[string]ed25519.PublicKey
	weights            *sybilprotection.Weights
	blocksByID         map[string]*models.Block
	epochEntityCounter map[epoch.Index]int

	sync.RWMutex
}

func NewTestFramework(test *testing.T) *TestFramework {
	tf := &TestFramework{
		test:               test,
		transactionsByID:   make(map[string]*ledger.TransactionMetadata),
		issuersByID:        make(map[string]ed25519.PublicKey),
		blocksByID:         make(map[string]*models.Block),
		epochEntityCounter: make(map[epoch.Index]int),
	}

	tf.weights = sybilprotection.NewWeights(mapdb.NewMapDB())
	tf.MutationFactory = NewEpochMutations(tf.weights, 0)

	return tf
}

func (t *TestFramework) CreateIssuer(alias string, issuerWeight ...int64) (issuer ed25519.PublicKey) {
	t.Lock()
	defer t.Unlock()

	if _, exists := t.issuersByID[alias]; exists {
		panic("issuer alias already exists")
	}

	issuer = ed25519.GenerateKeyPair().PublicKey

	t.issuersByID[alias] = issuer
	if len(issuerWeight) == 1 {
		t.weights.Update(identity.NewID(issuer), sybilprotection.NewWeight(issuerWeight[0], 0))
	}
	return issuer
}

func (t *TestFramework) Issuer(alias string) (issuer ed25519.PublicKey) {
	t.RLock()
	defer t.RUnlock()

	return t.issuersByID[alias]
}

func (t *TestFramework) CreateBlock(alias string, index epoch.Index, blockOpts ...options.Option[models.Block]) (block *models.Block) {
	t.Lock()
	defer t.Unlock()

	if t.blocksByID[alias] != nil {
		panic("block alias already exists")
	}

	block = models.NewBlock(append([]options.Option[models.Block]{
		models.WithIssuingTime(index.StartTime().Add(time.Duration(t.increaseEpochEntityCounter(index)) * time.Millisecond)),
		models.WithStrongParents(
			models.NewBlockIDs(models.EmptyBlockID),
		),
	}, blockOpts...)...)
	if err := block.DetermineID(); err != nil {
		panic(err)
	}

	t.blocksByID[alias] = block

	return block
}

func (t *TestFramework) Block(alias string) (block *models.Block) {
	t.RLock()
	defer t.RUnlock()

	return t.blocksByID[alias]
}

func (t *TestFramework) CreateTransaction(alias string, index epoch.Index) (metadata *ledger.TransactionMetadata) {
	t.Lock()
	defer t.Unlock()

	if _, exists := t.transactionsByID[alias]; exists {
		panic("transaction alias already exists")
	}

	var txID utxo.TransactionID
	require.NoError(t.test, txID.FromRandomness())
	metadata = ledger.NewTransactionMetadata(txID)
	metadata.SetInclusionEpoch(index)

	t.transactionsByID[alias] = metadata

	return metadata
}

func (t *TestFramework) Transaction(alias string) *ledger.TransactionMetadata {
	t.RLock()
	defer t.RUnlock()

	return t.transactionsByID[alias]
}

func (t *TestFramework) AddAcceptedBlock(alias string) (err error) {
	block := t.Block(alias)
	if block == nil {
		panic("block does not exist")
	}

	return t.MutationFactory.AddAcceptedBlock(block)
}

func (t *TestFramework) RemoveAcceptedBlock(alias string) (err error) {
	block := t.Block(alias)
	if block == nil {
		panic("block does not exist")
	}

	return t.MutationFactory.RemoveAcceptedBlock(block)
}

func (t *TestFramework) AddAcceptedTransaction(alias string) (err error) {
	tx := t.Transaction(alias)
	if tx == nil {
		panic("transaction does not exist")
	}

	return t.MutationFactory.AddAcceptedTransaction(tx)
}

func (t *TestFramework) RemoveAcceptedTransaction(alias string) (err error) {
	tx := t.Transaction(alias)
	if tx == nil {
		panic("transaction does not exist")
	}

	return t.MutationFactory.RemoveAcceptedTransaction(tx)
}

func (t *TestFramework) UpdateTransactionInclusion(alias string, oldEpoch, newEpoch epoch.Index) (err error) {
	tx := t.Transaction(alias)
	if tx == nil {
		panic("transaction does not exist")
	}

	return t.MutationFactory.UpdateTransactionInclusion(tx.ID(), oldEpoch, newEpoch)
}

func (t *TestFramework) AssertCommit(index epoch.Index, expectedBlocks []string, expectedTransactions []string, expectedValidators []string, expectedCumulativeWeight int64, optShouldError ...bool) {
	acceptedBlocks, acceptedTransactions, err := t.MutationFactory.Evict(index)
	if len(optShouldError) > 0 && optShouldError[0] {
		require.Error(t.test, err)
	}

	if acceptedBlocks == nil {
		require.Equal(t.test, len(expectedBlocks), 0)
	} else {
		require.Equal(t.test, len(expectedBlocks), acceptedBlocks.Size())
		for _, expectedBlock := range expectedBlocks {
			require.True(t.test, acceptedBlocks.Has(t.Block(expectedBlock).ID()))
		}
	}

	if acceptedTransactions == nil {
		require.Equal(t.test, len(expectedTransactions), 0)
	} else {
		require.Equal(t.test, len(expectedTransactions), acceptedTransactions.Size())
		for _, expectedTransaction := range expectedTransactions {
			require.True(t.test, acceptedTransactions.Has(t.Transaction(expectedTransaction).ID()))
		}
	}
}

func (t *TestFramework) increaseEpochEntityCounter(index epoch.Index) (nextBlockCounter int) {
	t.epochEntityCounter[index]++

	return t.epochEntityCounter[index]
}
