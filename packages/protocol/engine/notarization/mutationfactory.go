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

// MutationFactory is an in-memory data structure that enables the collection of mutations for uncommitted epochs.
type MutationFactory struct {
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

	sync.Mutex
}

// NewMutationFactory creates a new mutation factory.
func NewMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *MutationFactory) {
	return &MutationFactory{
		acceptedBlocksByEpoch:       memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactionsByEpoch: memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activeValidatorsByEpoch:     memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
		issuerBlocksByEpoch:         memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),

		latestCommittedIndex: lastCommittedEpoch,
	}
}

// AddAcceptedBlock adds the given block to the set of accepted blocks.
func (m *MutationFactory) AddAcceptedBlock(block *models.Block) (err error) {
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
func (m *MutationFactory) RemoveAcceptedBlock(block *models.Block) (err error) {
	m.Lock()
	defer m.Unlock()

	blockID := block.ID()
	if blockID.Index() <= m.latestCommittedIndex {
		return errors.Errorf("cannot add block %s: epoch with %d is already committed", blockID, blockID.Index())
	}

	m.acceptedBlocks(blockID.Index()).Delete(blockID)
	m.removeBlockByIssuer(blockID, block.IssuerID())

	return
}

// AddAcceptedTransaction adds the given transaction to the set of accepted transactions.
func (m *MutationFactory) AddAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
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
func (m *MutationFactory) RemoveAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
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
func (m *MutationFactory) UpdateTransactionInclusion(txID utxo.TransactionID, oldEpoch, newEpoch epoch.Index) (err error) {
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
func (m *MutationFactory) Commit(index epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], activeValidators *ads.Set[identity.ID]) {
	m.Lock()
	defer m.Unlock()

	if index <= m.latestCommittedIndex {
		panic("cannot commit epoch that is already committed")
	}

	defer m.evict(index)

	return m.acceptedBlocks(index), m.acceptedTransactions(index), m.activeValidators(index)
}

// acceptedBlocks returns the set of accepted blocks for the given epoch.
func (m *MutationFactory) acceptedBlocks(index epoch.Index, createIfMissing ...bool) *ads.Set[models.BlockID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedBlocksByEpoch.RetrieveOrCreate(index, newSet[models.BlockID]))
	}

	return lo.Return1(m.acceptedBlocksByEpoch.Get(index))
}

// acceptedTransactions returns the set of accepted transactions for the given epoch.
func (m *MutationFactory) acceptedTransactions(index epoch.Index, createIfMissing ...bool) *ads.Set[utxo.TransactionID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedTransactionsByEpoch.RetrieveOrCreate(index, newSet[utxo.TransactionID]))
	}

	return lo.Return1(m.acceptedTransactionsByEpoch.Get(index))
}

// activeValidators returns the set of active validators for the given epoch.
func (m *MutationFactory) activeValidators(index epoch.Index, createIfMissing ...bool) *ads.Set[identity.ID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.activeValidatorsByEpoch.RetrieveOrCreate(index, newSet[identity.ID]))
	}

	return lo.Return1(m.activeValidatorsByEpoch.Get(index))
}

// addBlockByIssuer adds the given block to the set of blocks issued by the given issuer.
func (m *MutationFactory) addBlockByIssuer(blockID models.BlockID, issuer identity.ID) {
	blocksByIssuer, isNewIssuer := m.issuerBlocksByEpoch.Get(blockID.Index(), true).RetrieveOrCreate(issuer, func() *set.AdvancedSet[models.BlockID] { return set.NewAdvancedSet[models.BlockID]() })
	if isNewIssuer {
		m.activeValidators(blockID.Index(), true).Add(issuer)
	}

	blocksByIssuer.Add(blockID)
}

// removeBlockByIssuer removes the given block from the set of blocks issued by the given issuer.
func (m *MutationFactory) removeBlockByIssuer(blockID models.BlockID, issuer identity.ID) {
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

// evict removes all data for epochs that are older than the given epoch.
func (m *MutationFactory) evict(index epoch.Index) {
	for i := m.latestCommittedIndex; i < index; i++ {
		m.acceptedBlocksByEpoch.Delete(index)
		m.acceptedTransactionsByEpoch.Delete(index)
		m.activeValidatorsByEpoch.Delete(index)
		m.issuerBlocksByEpoch.EvictEpoch(index)
	}
}

// newSet is a generic constructor for a new ads.Set.
func newSet[A constraints.Serializable]() *ads.Set[A] {
	return ads.NewSet[A](mapdb.NewMapDB())
}