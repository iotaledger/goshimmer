package booker

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type attachments struct {
	attachments *memstorage.Storage[utxo.TransactionID, *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]]
	evictionMap *memstorage.Storage[epoch.Index, set.Set[utxo.TransactionID]]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments: memstorage.New[utxo.TransactionID, *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]](),
		evictionMap: memstorage.New[epoch.Index, set.Set[utxo.TransactionID]](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *Block) {
	a.storeAttachment(txID, block)
	a.updateEvictionMap(block.ID().EpochIndex, txID)
}

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *LockableSlice[*Block]) bool {
			attachments = append(attachments, blocks.Slice()...)
			return true
		})
	}

	return
}

func (a *attachments) EvictEpoch(epochIndex epoch.Index) {
	if txIDs, exists := a.evictionMap.Get(epochIndex); exists {
		a.evictionMap.Delete(epochIndex)

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

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *LockableSlice[*Block]) bool {
			for _, block := range blocks.Slice() {
				if lowestTime.After(block.IssuingTime()) || lowestTime.IsZero() {
					lowestTime = block.IssuingTime()
					attachment = block
				}
			}
			return true
		})
	}

	return
}

func (a *attachments) getLatestAttachment(txID utxo.TransactionID) (attachment *Block) {
	highestTime := time.Time{}
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *LockableSlice[*Block]) bool {
			for _, block := range blocks.Slice() {
				if highestTime.Before(block.IssuingTime()) {
					highestTime = block.IssuingTime()
					attachment = block
				}
			}
			return true
		})
	}

	return
}

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *memstorage.Storage[epoch.Index, *LockableSlice[*Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.RetrieveOrCreate(txID, memstorage.New[epoch.Index, *LockableSlice[*Block]])
		return
	}

	storage, _ = a.attachments.Get(txID)

	return
}

func (a *attachments) updateEvictionMap(epochIndex epoch.Index, txID utxo.TransactionID) {
	txIDs, _ := a.evictionMap.RetrieveOrCreate(epochIndex, func() set.Set[utxo.TransactionID] {
		return set.New[utxo.TransactionID](true)
	})
	txIDs.Add(txID)
}
