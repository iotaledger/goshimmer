package notarization

import (
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type EpochMutations struct {
	acceptedBlocks       *memstorage.Storage[epoch.Index, *ads.Set[models.BlockID]]
	acceptedTransactions *memstorage.Storage[epoch.Index, *ads.Set[utxo.TransactionID]]
	activeValidator      *memstorage.Storage[epoch.Index, *ads.Set[identity.ID]]
}

func NewEpochMutations() (newEpochMutations *EpochMutations) {
	return &EpochMutations{
		acceptedBlocks:       memstorage.New[epoch.Index, *ads.Set[models.BlockID]](),
		acceptedTransactions: memstorage.New[epoch.Index, *ads.Set[utxo.TransactionID]](),
		activeValidator:      memstorage.New[epoch.Index, *ads.Set[identity.ID]](),
	}
}

func (u *EpochMutations) AcceptedBlocks(index epoch.Index, createIfMissing ...bool) *ads.Set[models.BlockID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(u.acceptedBlocks.RetrieveOrCreate(index, newADSSet[models.BlockID]))
	}

	return lo.Return1(u.acceptedBlocks.Get(index))
}

func (u *EpochMutations) AcceptedTransactions(index epoch.Index, createIfMissing ...bool) *ads.Set[utxo.TransactionID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(u.acceptedTransactions.RetrieveOrCreate(index, newADSSet[utxo.TransactionID]))
	}

	return lo.Return1(u.acceptedTransactions.Get(index))
}

func (u *EpochMutations) ActiveValidators(index epoch.Index, createIfMissing ...bool) *ads.Set[identity.ID] {
	if len(createIfMissing) > 0 && createIfMissing[0] {
		return lo.Return1(u.activeValidator.RetrieveOrCreate(index, newADSSet[identity.ID]))
	}

	return lo.Return1(u.activeValidator.Get(index))
}

func (u *EpochMutations) Commit(index epoch.Index) (acceptedBlocks *ads.Set[models.BlockID], acceptedTransactions *ads.Set[utxo.TransactionID], activeValidators *ads.Set[identity.ID]) {
	defer u.evict(index)

	return u.AcceptedBlocks(index), u.AcceptedTransactions(index), u.ActiveValidators(index)
}

func (u *EpochMutations) evict(index epoch.Index) {
	u.acceptedBlocks.Delete(index)
	u.acceptedTransactions.Delete(index)
	u.activeValidator.Delete(index)
}

func newADSSet[A constraints.Serializable]() *ads.Set[A] {
	return ads.NewSet[A](mapdb.NewMapDB())
}
