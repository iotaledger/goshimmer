package notarization

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type mutationFactory struct {
	acceptedBlockByEpoch           *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]
	acceptedTransactionsByEpoch    *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]
	activeNodesByEpoch             *memstorage.Storage[epoch.Index, *ads.Set[identity.ID]]
	acceptedBlocksByEpochAndIssuer *memstorage.EpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]]
	lastCommittedEpochIndex        epoch.Index
}

func newMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *mutationFactory) {
	return &mutationFactory{
		acceptedBlockByEpoch:           memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactionsByEpoch:    memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activeNodesByEpoch:             memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
		acceptedBlocksByEpochAndIssuer: memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),
		lastCommittedEpochIndex:        lastCommittedEpoch,
	}
}

func (m *mutationFactory) commit(ei epoch.Index) (tangleRoot, stateMutationRoot, activityRoot types.Identifier) {
	return lo.Return1(m.acceptedBlockByEpoch.RetrieveOrCreate(ei, func() *ads.Set[models.BlockID] { return ads.NewSet[models.BlockID](mapdb.NewMapDB()) })).Root(),
		lo.Return1(m.acceptedTransactionsByEpoch.RetrieveOrCreate(ei, func() *ads.Set[utxo.TransactionID] { return ads.NewSet[utxo.TransactionID](mapdb.NewMapDB()) })).Root(),
		lo.Return1(m.activeNodesByEpoch.RetrieveOrCreate(ei, func() *ads.Set[identity.ID] { return ads.NewSet[identity.ID](mapdb.NewMapDB()) })).Root()
}

func (m *mutationFactory) addAcceptedBlock(id identity.ID, blockID models.BlockID) {
	acceptedBlocksByIssuerID, created := m.acceptedBlocksByEpochAndIssuer.Get(blockID.Index(), true).RetrieveOrCreate(id, func() *set.AdvancedSet[models.BlockID] {
		return set.NewAdvancedSet[models.BlockID]()
	})
	if created {
		lo.Return1(m.activeNodesByEpoch.RetrieveOrCreate(blockID.Index(), func() *ads.Set[identity.ID] {
			return ads.NewSet[identity.ID](mapdb.NewMapDB())
		})).Add(id)
	}

	acceptedBlocksByIssuerID.Add(blockID)
}

func (m *mutationFactory) removeAcceptedBlock(id identity.ID, blockID models.BlockID) {
	acceptedBlocksByIssuer := m.acceptedBlocksByEpochAndIssuer.Get(blockID.Index(), false)
	if acceptedBlocksByIssuer == nil {
		return
	}

	acceptedBlocks, exists := acceptedBlocksByIssuer.Get(id)
	if !exists {
		return
	}

	if acceptedBlocks.Delete(blockID) && acceptedBlocks.IsEmpty() {
		if activeNodesForEpoch, exists := m.activeNodesByEpoch.Get(blockID.Index()); exists {
			activeNodesForEpoch.Delete(id)
		}
	}
}

func (m *mutationFactory) addAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	acceptedTransactions, _ := m.acceptedTransactionsByEpoch.RetrieveOrCreate(ei, func() *ads.Set[utxo.TransactionID] {
		return ads.NewSet[utxo.TransactionID](mapdb.NewMapDB())
	})
	acceptedTransactions.Add(txID)
}

func (m *mutationFactory) removeAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	acceptedTransactions, exists := m.acceptedTransactionsByEpoch.Get(ei)
	if !exists {
		return
	}
	acceptedTransactions.Delete(txID)
}

func (m *mutationFactory) evict(ei epoch.Index) {
	m.acceptedBlockByEpoch.Delete(ei)
	m.acceptedTransactionsByEpoch.Delete(ei)
	m.activeNodesByEpoch.Delete(ei)
	m.acceptedBlocksByEpochAndIssuer.EvictEpoch(ei)
}
