package notarization

import (
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type MutationFactory struct {
	*EpochMutations

	lastCommittedEpochIndex epoch.Index

	blocksByEpochAndIssuer *memstorage.EpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]]
}

func NewMutationFactory(lastCommittedEpoch epoch.Index) (newMutationFactory *MutationFactory) {
	return &MutationFactory{
		lastCommittedEpochIndex: lastCommittedEpoch,
		EpochMutations:               NewEpochMutations(),
		blocksByEpochAndIssuer:  memstorage.NewEpochStorage[identity.ID, *set.AdvancedSet[models.BlockID]](),
	}
}

func (m *MutationFactory) Commit(ei epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], activeValidators *ads.Set[identity.ID]) {
	defer m.blocksByEpochAndIssuer.EvictEpoch(ei)

	return m.EpochMutations.Commit(ei)
}

func (m *MutationFactory) AddAcceptedBlock(issuer identity.ID, blockID models.BlockID) {
	m.AcceptedBlocks(blockID.Index(), true).Add(blockID)

	m.addAcceptedBlockOfIssuer(blockID, issuer)
}

func (m *MutationFactory) RemoveAcceptedBlock(issuer identity.ID, blockID models.BlockID) {
	m.AcceptedBlocks(blockID.Index()).Delete(blockID)

	m.removeAcceptedBlockOfIssuer(blockID, issuer)
}

func (m *MutationFactory) AddAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	m.AcceptedTransactions(ei, true).Add(txID)
}

func (m *MutationFactory) RemoveAcceptedTransaction(ei epoch.Index, txID utxo.TransactionID) {
	m.AcceptedTransactions(ei, false).Delete(txID)
}

func (m *MutationFactory) addAcceptedBlockOfIssuer(blockID models.BlockID, issuer identity.ID) {
	blocksByIssuer, isNewIssuer := m.blocksByEpochAndIssuer.Get(blockID.Index(), true).RetrieveOrCreate(issuer, func() *set.AdvancedSet[models.BlockID] { return set.NewAdvancedSet[models.BlockID]() })
	if isNewIssuer {
		m.ActiveValidators(blockID.Index(), true).Add(issuer)
	}

	blocksByIssuer.Add(blockID)
}

func (m *MutationFactory) removeAcceptedBlockOfIssuer(blockID models.BlockID, issuer identity.ID) {
	epochBlocks := m.blocksByEpochAndIssuer.Get(blockID.Index())
	if epochBlocks == nil {
		return
	}

	blocksByIssuer, exists := epochBlocks.Get(issuer)
	if !exists  || !blocksByIssuer.Delete(blockID) || !blocksByIssuer.IsEmpty() {
		return
	}

	m.ActiveValidators(blockID.Index()).Delete(issuer)
}