package notarization

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// EpochMutations is an in-memory data structure that enables the collection of mutations for uncommitted epochs.
type EpochMutations struct {
	weights *sybilprotection.Weights

	Events *EpochMutationsEvents

	// acceptedBlocksByEpoch stores the accepted blocks per epoch.
	acceptedBlocksByEpoch *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]

	// acceptedTransactionsByEpoch stores the accepted transactions per epoch.
	acceptedTransactionsByEpoch *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]

	// attestationByEpoch stores the attestation per epoch.
	attestationsByEpoch *memstorage.Storage[epoch.Index, *Attestations]

	// latestCommittedIndex stores the index of the latest committed epoch.
	latestCommittedIndex epoch.Index

	evictionMutex sync.RWMutex

	// lastCommittedEpochCumulativeWeight stores the cumulative weight of the last committed epoch
	lastCommittedEpochCumulativeWeight uint64
}

// NewEpochMutations creates a new EpochMutations instance.
func NewEpochMutations(weights *sybilprotection.Weights, lastCommittedEpoch epoch.Index) (newMutationFactory *EpochMutations) {
	return &EpochMutations{
		Events:                      NewEpochMutationsEvents(),
		weights:                     weights,
		acceptedBlocksByEpoch:       memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactionsByEpoch: memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		attestationsByEpoch:         memstorage.New[epoch.Index, *Attestations](),
		latestCommittedIndex:        lastCommittedEpoch,
	}
}

// AddAcceptedBlock adds the given block to the set of accepted blocks.
func (m *EpochMutations) AddAcceptedBlock(block *models.Block) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: epoch with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index(), true).Add(blockID)

	lo.Return1(m.attestationsByEpoch.RetrieveOrCreate(block.ID().Index(), func() *Attestations {
		return NewAttestations(m.weights)
	})).Add(block)

	return
}

// RemoveAcceptedBlock removes the given block from the set of accepted blocks.
func (m *EpochMutations) RemoveAcceptedBlock(block *models.Block) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: epoch with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index()).Delete(blockID)

	if attestations, exists := m.attestationsByEpoch.Get(blockID.Index()); exists {
		attestations.Delete(block)
	}

	m.Events.AcceptedBlockRemoved.Trigger(blockID)
	return
}

// AddAcceptedTransaction adds the given transaction to the set of accepted transactions.
func (m *EpochMutations) AddAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, true).Add(metadata.ID())

	return
}

// RemoveAcceptedTransaction removes the given transaction from the set of accepted transactions.
func (m *EpochMutations) RemoveAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, false).Delete(metadata.ID())

	return
}

// UpdateTransactionInclusion moves a transaction from a later epoch to the given epoch.
func (m *EpochMutations) UpdateTransactionInclusion(txID utxo.TransactionID, oldEpoch, newEpoch epoch.Index) (err error) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	if newEpoch >= oldEpoch {
		return
	}

	if oldEpoch <= m.latestCommittedIndex || newEpoch <= m.latestCommittedIndex {
		return errors.Errorf("inclusion time of transaction changed for already committed epoch: previous Index %d, new Index %d", oldEpoch, newEpoch)
	}

	m.acceptedTransactions(oldEpoch, false).Delete(txID)
	m.acceptedTransactions(newEpoch, true).Add(txID)

	return
}

// Evict evicts the given epoch and returns the corresponding mutation sets.
func (m *EpochMutations) Evict(index epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], attestations *Attestations, err error) {
	m.evictionMutex.Lock()
	defer m.evictionMutex.Unlock()

	if index <= m.latestCommittedIndex {
		return nil, nil, nil, errors.Errorf("cannot commit epoch %d: already committed", index)
	}

	defer m.evictUntil(index)

	return m.acceptedBlocks(index), m.acceptedTransactions(index), lo.Return1(m.attestationsByEpoch.Get(index)), nil
}

// Attestations returns the attestations for the given epoch.
func (m *EpochMutations) Attestations(index epoch.Index) (epochAttestations *Attestations) {
	m.evictionMutex.RLock()
	defer m.evictionMutex.RUnlock()

	return lo.Return1(m.attestationsByEpoch.RetrieveOrCreate(index, func() *Attestations {
		return NewAttestations(m.weights)
	}))
}

// acceptedBlocks returns the set of accepted blocks for the given epoch.
func (m *EpochMutations) acceptedBlocks(index epoch.Index, createIfMissing ...bool) *ads.Set[models.BlockID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedBlocksByEpoch.RetrieveOrCreate(index, newSet[models.BlockID]))
	}

	return lo.Return1(m.acceptedBlocksByEpoch.Get(index))
}

// acceptedTransactions returns the set of accepted transactions for the given epoch.
func (m *EpochMutations) acceptedTransactions(index epoch.Index, createIfMissing ...bool) *ads.Set[utxo.TransactionID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedTransactionsByEpoch.RetrieveOrCreate(index, newSet[utxo.TransactionID]))
	}

	return lo.Return1(m.acceptedTransactionsByEpoch.Get(index))
}

// evictUntil removes all data for epochs that are older than the given epoch.
func (m *EpochMutations) evictUntil(index epoch.Index) {
	for i := m.latestCommittedIndex + 1; i <= index; i++ {
		m.acceptedBlocksByEpoch.Delete(i)
		m.acceptedTransactionsByEpoch.Delete(i)
		attestations, exists := m.attestationsByEpoch.Get(i)
		if exists {
			attestations.Detach()
		}
		m.attestationsByEpoch.Delete(i)
	}

	m.latestCommittedIndex = index
}

// newSet is a generic constructor for a new ads.Set.
func newSet[A constraints.Serializable]() *ads.Set[A] {
	return ads.NewSet[A](mapdb.NewMapDB())
}
