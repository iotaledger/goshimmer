package booker

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

type attachments struct {
	attachments     *memstorage.Storage[utxo.TransactionID, *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]]
	pruningMap      *memstorage.Storage[epoch.Index, set.Set[utxo.TransactionID]]
	maxDroppedEpoch epoch.Index

	sync.RWMutex
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments:     memstorage.New[utxo.TransactionID, *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]](),
		pruningMap:      memstorage.New[epoch.Index, set.Set[utxo.TransactionID]](),
		maxDroppedEpoch: -1,
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *Block) {
	a.RLock()
	defer a.RUnlock()

	if block.ID().EpochIndex <= a.maxDroppedEpoch {
		return
	}

	a.storeAttachment(txID, block)
	a.updatePruningMap(block.ID().EpochIndex, txID)
}

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*Block) {
	a.RLock()
	defer a.RUnlock()

	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *LockableSlice[*Block]) bool {
			attachments = append(attachments, blocks.Slice()...)
			return true
		})
	}

	return
}

func (a *attachments) Prune(epochIndex epoch.Index) {
	a.Lock()
	defer a.Unlock()

	for a.maxDroppedEpoch < epochIndex {
		a.maxDroppedEpoch++
		a.dropEpoch(a.maxDroppedEpoch)
	}
}

func (a *attachments) dropEpoch(epochIndex epoch.Index) {
	if txIDs, exists := a.pruningMap.Get(epochIndex); exists {
		a.pruningMap.Delete(epochIndex)

		txIDs.ForEach(func(txID utxo.TransactionID) {
			if attachmentsOfTX := a.storage(txID, false); attachmentsOfTX != nil && attachmentsOfTX.Delete(epochIndex) && attachmentsOfTX.Size() == 0 {
				a.attachments.Delete(txID)
			}
		})
	}
}

func (a *attachments) storeAttachment(txID utxo.TransactionID, block *Block) {
	attachmentsOfEpoch, _ := a.storage(txID, true).RetrieveOrCreate(block.ID().EpochIndex, func() *LockableSlice[*Block] {
		return NewLockableSlice[*Block]()
	})
	attachmentsOfEpoch.Append(block)
}

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.RetrieveOrCreate(txID, memstorage.New[epoch.Index, *LockableSlice[*Block]])
		return
	}

	storage, _ = a.attachments.Get(txID)

	return
}

func (a *attachments) updatePruningMap(epochIndex epoch.Index, txID utxo.TransactionID) {
	txIDs, _ := a.pruningMap.RetrieveOrCreate(epochIndex, func() set.Set[utxo.TransactionID] {
		return set.New[utxo.TransactionID](true)
	})
	txIDs.Add(txID)
}
