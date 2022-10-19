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
	tangleTreeByEpoch              *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]
	mutationTreeByEpoch            *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]
	activityTreeByEpoch            *memstorage.Storage[epoch.Index, *ads.Set[identity.ID]]
	acceptedBlocksByEpochAndIssuer *memstorage.EpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]]
	lastCommittedEpochIndex        epoch.Index
}

func newMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *mutationFactory) {
	return &mutationFactory{
		tangleTreeByEpoch:              memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		mutationTreeByEpoch:            memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activityTreeByEpoch:            memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
		acceptedBlocksByEpochAndIssuer: memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),
		lastCommittedEpochIndex:        lastCommittedEpoch,
	}
}

func (m *mutationFactory) commit(ei epoch.Index) (tangleRoot, stateMutationRoot, activityRoot types.Identifier) {
	return lo.Return1(m.tangleTreeByEpoch.RetrieveOrCreate(ei, func() *ads.Set[models.BlockID] { return ads.NewSet[models.BlockID](mapdb.NewMapDB()) })).Root(),
		lo.Return1(m.mutationTreeByEpoch.RetrieveOrCreate(ei, func() *ads.Set[utxo.TransactionID] { return ads.NewSet[utxo.TransactionID](mapdb.NewMapDB()) })).Root(),
		lo.Return1(m.activityTreeByEpoch.RetrieveOrCreate(ei, func() *ads.Set[identity.ID] { return ads.NewSet[identity.ID](mapdb.NewMapDB()) })).Root()
}

func (m *mutationFactory) addAcceptedBlock(id identity.ID, blockID models.BlockID) {
	// Adds the block issuer as active.
	acceptedBlocksByIssuerID, created := m.acceptedBlocksByEpochAndIssuer.Get(blockID.Index(), true).RetrieveOrCreate(id, func() *set.AdvancedSet[models.BlockID] {
		return set.NewAdvancedSet[models.BlockID]()
	})
	if created {
		lo.Return1(m.activityTreeByEpoch.RetrieveOrCreate(blockID.Index(), func() *ads.Set[identity.ID] {
			return ads.NewSet[identity.ID](mapdb.NewMapDB())
		})).Add(id)
	}

	acceptedBlocksByIssuerID.Add(blockID)

	// Add to TangleRoot
	lo.Return1(m.tangleTreeByEpoch.RetrieveOrCreate(blockID.Index(), func() *ads.Set[models.BlockID] {
		return ads.NewSet[models.BlockID](mapdb.NewMapDB())
	})).Add(blockID)
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

	// Remove the block issuer as active if no blocks left in the map.
	if acceptedBlocks.Delete(blockID) && acceptedBlocks.IsEmpty() {
		if activeNodesForEpoch, exists := m.activityTreeByEpoch.Get(blockID.Index()); exists {
			activeNodesForEpoch.Delete(id)
		}
	}

	// Remove from TangleRoot
	if tangleTree, exists := m.tangleTreeByEpoch.Get(blockID.Index()); exists {
		tangleTree.Delete(blockID)
	}
}

func (m *mutationFactory) addAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	acceptedTransactions, _ := m.mutationTreeByEpoch.RetrieveOrCreate(ei, func() *ads.Set[utxo.TransactionID] {
		return ads.NewSet[utxo.TransactionID](mapdb.NewMapDB())
	})
	acceptedTransactions.Add(txID)
}

func (m *mutationFactory) hasAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) (has bool) {
	acceptedTransactions, exists := m.mutationTreeByEpoch.Get(ei)
	return exists && acceptedTransactions.Has(txID)
}

func (m *mutationFactory) removeAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	if acceptedTransactions, exists := m.mutationTreeByEpoch.Get(ei); exists {
		acceptedTransactions.Delete(txID)
	}
}

func (m *mutationFactory) evict(ei epoch.Index) {
	m.tangleTreeByEpoch.Delete(ei)
	m.mutationTreeByEpoch.Delete(ei)
	m.activityTreeByEpoch.Delete(ei)
	m.acceptedBlocksByEpochAndIssuer.EvictEpoch(ei)
}
