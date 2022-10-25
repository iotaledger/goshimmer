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

type MutationFactory struct {
	acceptedBlocksByEpoch       *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]
	acceptedTransactionsByEpoch *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]
	activeValidatorsByEpoch     *memstorage.Storage[epoch.Index, *ads.Set[identity.ID]]
	issuerBlocksByEpoch         *memstorage.EpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]]

	latestCommittedIndex epoch.Index

	sync.RWMutex
}

func NewMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *MutationFactory) {
	return &MutationFactory{
		acceptedBlocksByEpoch:       memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactionsByEpoch: memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activeValidatorsByEpoch:     memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
		issuerBlocksByEpoch:         memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),
	}
}

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

func (m *MutationFactory) AddAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, true).Add(metadata.ID())

	return
}

func (m *MutationFactory) RemoveAcceptedTransaction(metadata *ledger.TransactionMetadata) (err error) {
	epochIndex := epoch.IndexFromTime(metadata.InclusionTime())
	if epochIndex <= m.latestCommittedIndex {
		return errors.Errorf("transaction %s accepted with issuing time %s in already committed epoch %d", metadata.ID(), metadata.InclusionTime(), epochIndex)
	}

	m.acceptedTransactions(epochIndex, false).Delete(metadata.ID())

	return
}

func (m *MutationFactory) Commit(index epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], activeValidators *ads.Set[identity.ID]) {
	defer m.evict(index)

	return m.acceptedBlocks(index), m.acceptedTransactions(index), m.activeValidators(index)
}

func (m *MutationFactory) acceptedBlocks(index epoch.Index, createIfMissing ...bool) *ads.Set[models.BlockID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedBlocksByEpoch.RetrieveOrCreate(index, newADSSet[models.BlockID]))
	}

	return lo.Return1(m.acceptedBlocksByEpoch.Get(index))
}

func (m *MutationFactory) acceptedTransactions(index epoch.Index, createIfMissing ...bool) *ads.Set[utxo.TransactionID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.acceptedTransactionsByEpoch.RetrieveOrCreate(index, newADSSet[utxo.TransactionID]))
	}

	return lo.Return1(m.acceptedTransactionsByEpoch.Get(index))
}

func (m *MutationFactory) activeValidators(index epoch.Index, createIfMissing ...bool) *ads.Set[identity.ID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(m.activeValidatorsByEpoch.RetrieveOrCreate(index, newADSSet[identity.ID]))
	}

	return lo.Return1(m.activeValidatorsByEpoch.Get(index))
}

func (m *MutationFactory) addBlockByIssuer(blockID models.BlockID, issuer identity.ID) {
	blocksByIssuer, isNewIssuer := m.issuerBlocksByEpoch.Get(blockID.Index(), true).RetrieveOrCreate(issuer, func() *set.AdvancedSet[models.BlockID] { return set.NewAdvancedSet[models.BlockID]() })
	if isNewIssuer {
		m.activeValidators(blockID.Index(), true).Add(issuer)
	}

	blocksByIssuer.Add(blockID)
}

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

func (m *MutationFactory) evict(index epoch.Index) {
	m.acceptedBlocksByEpoch.Delete(index)
	m.acceptedTransactionsByEpoch.Delete(index)
	m.activeValidatorsByEpoch.Delete(index)
	m.issuerBlocksByEpoch.EvictEpoch(index)
}

func newADSSet[A constraints.Serializable]() *ads.Set[A] {
	return ads.NewSet[A](mapdb.NewMapDB())
}
