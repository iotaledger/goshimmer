package slotnotarization

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	MutationFactory  *SlotMutations
	slotTimeProvider *slot.TimeProvider

	test              *testing.T
	transactionsByID  map[string]*mempool.TransactionMetadata
	issuersByID       map[string]ed25519.PublicKey
	weights           *sybilprotection.Weights
	blocksByID        map[string]*models.Block
	slotEntityCounter map[slot.Index]int

	sync.RWMutex
}

func NewTestFramework(test *testing.T, slotTimeProvider *slot.TimeProvider) *TestFramework {
	tf := &TestFramework{
		test:              test,
		slotTimeProvider:  slotTimeProvider,
		transactionsByID:  make(map[string]*mempool.TransactionMetadata),
		issuersByID:       make(map[string]ed25519.PublicKey),
		blocksByID:        make(map[string]*models.Block),
		slotEntityCounter: make(map[slot.Index]int),
	}

	tf.weights = sybilprotection.NewWeights(mapdb.NewMapDB())
	tf.MutationFactory = NewSlotMutations(tf.weights, 0)

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

func (t *TestFramework) CreateBlock(alias string, index slot.Index, blockOpts ...options.Option[models.Block]) (block *models.Block) {
	t.Lock()
	defer t.Unlock()

	if t.blocksByID[alias] != nil {
		panic("block alias already exists")
	}

	block = models.NewBlock(append([]options.Option[models.Block]{
		models.WithIssuingTime(t.slotTimeProvider.StartTime(index).Add(time.Duration(t.increaseSlotEntityCounter(index)) * time.Millisecond)),
		models.WithStrongParents(
			models.NewBlockIDs(models.EmptyBlockID),
		),
	}, blockOpts...)...)
	if err := block.DetermineID(t.slotTimeProvider); err != nil {
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

func (t *TestFramework) CreateTransaction(alias string, index slot.Index) (metadata *mempool.TransactionMetadata) {
	t.Lock()
	defer t.Unlock()

	if _, exists := t.transactionsByID[alias]; exists {
		panic("transaction alias already exists")
	}

	var txID utxo.TransactionID
	require.NoError(t.test, txID.FromRandomness())
	metadata = mempool.NewTransactionMetadata(txID)
	metadata.SetInclusionSlot(index)

	t.transactionsByID[alias] = metadata

	return metadata
}

func (t *TestFramework) Transaction(alias string) *mempool.TransactionMetadata {
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

func (t *TestFramework) UpdateTransactionInclusion(alias string, oldSlot, newSlot slot.Index) (err error) {
	tx := t.Transaction(alias)
	if tx == nil {
		panic("transaction does not exist")
	}

	return t.MutationFactory.UpdateTransactionInclusion(tx.ID(), oldSlot, newSlot)
}

func (t *TestFramework) AssertCommit(index slot.Index, expectedBlocks []string, expectedTransactions []string, expectedValidators []string, expectedCumulativeWeight int64, optShouldError ...bool) {
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

func (t *TestFramework) increaseSlotEntityCounter(index slot.Index) (nextBlockCounter int) {
	t.slotEntityCounter[index]++

	return t.slotEntityCounter[index]
}
