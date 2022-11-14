package notarization

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// EpochMutations is an in-memory data structure that enables the collection of mutations for uncommitted epochs.
type EpochMutations struct {
	validatorWeightFunc func(identity.ID) (int64, bool)

	Events *EpochMutationsEvents

	// acceptedBlocksByEpoch stores the accepted blocks per epoch.
	acceptedBlocksByEpoch *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]

	// acceptedTransactionsByEpoch stores the accepted transactions per epoch.
	acceptedTransactionsByEpoch *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]

	// activeValidatorsByEpoch stores the active validators per epoch.
	activeValidatorsByEpoch *memstorage.Storage[epoch.Index, *ads.Set[identity.ID]]

	// issuerBlocksByEpoch stores the blocks issued by a validator per epoch.
	issuerBlocksByEpoch *memstorage.EpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]]

	// latestCommittedIndex stores the index of the latest committed epoch.
	latestCommittedIndex epoch.Index

	// lastCommittedEpochCumulativeWeight stores the cumulative weight of the last committed epoch
	lastCommittedEpochCumulativeWeight uint64

	sync.Mutex
}

// NewEpochMutations creates a new EpochMutations instance.
func NewEpochMutations(validatorWeightFunc func(id identity.ID) (int64, bool), lastCommittedEpoch epoch.Index) (newMutationFactory *EpochMutations) {
	return &EpochMutations{
		Events:                      NewEpochMutationsEvents(),
		validatorWeightFunc: validatorWeightFunc,

		acceptedBlocksByEpoch:       memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactionsByEpoch: memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activeValidatorsByEpoch:     memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
		issuerBlocksByEpoch:         memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),

		latestCommittedIndex: lastCommittedEpoch,
	}
}

// AddAcceptedBlock adds the given block to the set of accepted blocks.
func (m *EpochMutations) AddAcceptedBlock(block *models.Block) (err error) {
	m.Lock()
	defer m.Unlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: epoch with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index(), true).Add(blockID)
	m.addBlockByIssuer(blockID, block.IssuerID())

	return
}

// RemoveAcceptedBlock removes the given block from the set of accepted blocks.
func (m *EpochMutations) RemoveAcceptedBlock(block *models.Block) (err error) {
	m.Lock()
	defer m.Unlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: epoch with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index()).Delete(blockID)
	m.removeBlockByIssuer(blockID, block.IssuerID())

	m.Events.AcceptedBlockRemoved.Trigger(blockID)
	return
}

// TotalAcceptedBlocks returns the number of accepted blocks of given epoch.
func (m *EpochMutations) TotalAcceptedBlocks(ei epoch.Index) (total int) {
	m.Lock()
	defer m.Unlock()

	if m.acceptedBlocks(ei) == nil {
		return 0
	}
	return m.acceptedBlocks(ei).Size()
}

// TotalAcceptedTransactions returns the number of accepted transactions of given epoch.
func (m *EpochMutations) TotalAcceptedTransactions(ei epoch.Index) (total int) {
	m.Lock()
	defer m.Unlock()

	if m.acceptedTransactions(ei) == nil {
		return 0
	}
	return m.acceptedTransactions(ei).Size()
}

// TotalActiveValidators returns the number of active validators of given epoch.
func (m *EpochMutations) TotalActiveValidators(ei epoch.Index) (total int) {
	m.Lock()
	defer m.Unlock()

	if m.activeValidators(ei) == nil {
		return 0
	}
	return m.activeValidators(ei).Size()
}

// AddAcceptedTransaction adds the given transaction to the set of accepted transactions.
func (m *EpochMutations) AddAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	m.Lock()
	defer m.Unlock()

	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, true).Add(metadata.ID())
	return
}

// RemoveAcceptedTransaction removes the given transaction from the set of accepted transactions.
func (m *EpochMutations) RemoveAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	m.Lock()
	defer m.Unlock()

	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, false).Delete(metadata.ID())

	return
}

// UpdateTransactionInclusion moves a transaction from a later epoch to the given epoch.
func (m *EpochMutations) UpdateTransactionInclusion(txID utxo.TransactionID, oldEpoch, newEpoch epoch.Index) (err error) {
	m.Lock()
	defer m.Unlock()

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

// Commit evicts the given epoch and returns the corresponding mutation sets.
func (m *EpochMutations) Commit(index epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], activeValidators *ads.Set[identity.ID], cumulativeWeight uint64, err error) {
	m.Lock()
	defer m.Unlock()

	if index <= m.latestCommittedIndex {
		return nil, nil, nil, 0, errors.Errorf("cannot commit epoch %d: already committed", index)
	}

	defer m.evictUntil(index)

	m.lastCommittedEpochCumulativeWeight += m.epochWeight(index)

	return m.acceptedBlocks(index), m.acceptedTransactions(index), m.activeValidators(index), m.lastCommittedEpochCumulativeWeight, nil
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

// activeValidators returns the set of active validators for the given epoch.
func (m *EpochMutations) activeValidators(index epoch.Index, createIfMissing ...bool) *ads.Set[identity.ID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.activeValidatorsByEpoch.RetrieveOrCreate(index, newSet[identity.ID]))
	}

	return lo.Return1(m.activeValidatorsByEpoch.Get(index))
}

func (m *EpochMutations) epochWeight(index epoch.Index) (epochWeight uint64) {
	storage := m.issuerBlocksByEpoch.Get(index, false)
	if storage == nil {
		return 0
	}
	storage.ForEachKey(func(issuerID identity.ID) bool {
		if weight, exists := m.validatorWeightFunc(issuerID); exists {
			epochWeight += uint64(weight)
		}
		return true
	})

	return epochWeight
}

// addBlockByIssuer adds the given block to the set of blocks issued by the given issuer.
func (m *EpochMutations) addBlockByIssuer(blockID models.BlockID, issuer identity.ID) {
	blocksByIssuer, isNewIssuer := m.issuerBlocksByEpoch.Get(blockID.Index(), true).RetrieveOrCreate(issuer, func() *set.AdvancedSet[models.BlockID] { return set.NewAdvancedSet[models.BlockID]() })
	if isNewIssuer {
		m.activeValidators(blockID.Index(), true).Add(issuer)
	}

	blocksByIssuer.Add(blockID)
}

// removeBlockByIssuer removes the given block from the set of blocks issued by the given issuer.
func (m *EpochMutations) removeBlockByIssuer(blockID models.BlockID, issuer identity.ID) {
	epochBlocks := m.issuerBlocksByEpoch.Get(blockID.Index())
	if epochBlocks == nil {
		return
	}

	blocksByIssuer, exists := epochBlocks.Get(issuer)
	if !exists || !blocksByIssuer.Delete(blockID) || !blocksByIssuer.IsEmpty() {
		return
	}

	m.activeValidators(blockID.Index()).Delete(issuer)
}

// evictUntil removes all data for epochs that are older than the given epoch.
func (m *EpochMutations) evictUntil(index epoch.Index) {
	for i := m.latestCommittedIndex + 1; i <= index; i++ {
		m.acceptedBlocksByEpoch.Delete(i)
		m.acceptedTransactionsByEpoch.Delete(i)
		m.activeValidatorsByEpoch.Delete(i)
		m.issuerBlocksByEpoch.Evict(i)
	}

	m.latestCommittedIndex = index
}

// newSet is a generic constructor for a new ads.Set.
func newSet[A constraints.Serializable]() *ads.Set[A] {
	return ads.NewSet[A](mapdb.NewMapDB())
}
