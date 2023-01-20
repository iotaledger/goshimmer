package booker

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type attachments struct {
	attachments *memstorage.Storage[utxo.TransactionID, *memstorage.Storage[epoch.Index, set.Set[*Block]]]
	evictionMap *memstorage.Storage[epoch.Index, set.Set[utxo.TransactionID]]

	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments: memstorage.New[utxo.TransactionID, *memstorage.Storage[epoch.Index, set.Set[*Block]]](),
		evictionMap: memstorage.New[epoch.Index, set.Set[utxo.TransactionID]](),
		mutex:       syncutils.NewDAGMutex[utxo.TransactionID](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *Block) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	a.storeAttachment(txID, block)
	a.updateEvictionMap(block.ID().EpochIndex, txID)
}

func (a *attachments) DeleteAttachment(txID utxo.TransactionID, block *Block) (lastAttachmentDeleted bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	storage := a.storage(txID, false)
	if storage == nil {
		// zero attachments registered
		return false
	}
	attachmentsOfEpoch, exists := storage.Get(block.ID().Index())
	if !exists {
		// there is no attachments of the transaction in the epoch already, so no need to continue
		return false
	}

	attachmentsOfEpoch.Delete(block)
	if attachmentsOfEpoch.Size() > 0 {
		// there are still some attachments of the transaction in the epoch, so don't continue
		return false
	}

	// delete attachments storage for the given epoch and update eviction map
	storage.Delete(block.ID().Index())
	if txIDs, evictionMapEntryExists := a.evictionMap.Get(block.ID().Index()); evictionMapEntryExists {
		txIDs.Delete(txID)
	}

	if storage.Size() > 0 {
		// there are still attachments of the transaction in other epochs
		return false
	}

	// delete transaction's attachment storage
	a.attachments.Delete(txID)

	return true
}

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks set.Set[*Block]) bool {
			blocks.ForEach(func(block *Block) {
				attachments = append(attachments, block)
			})
			return true
		})
	}

	return
}

func (a *attachments) Evict(epochIndex epoch.Index) {
	if txIDs, exists := a.evictionMap.Get(epochIndex); exists {
		a.evictionMap.Delete(epochIndex)

		txIDs.ForEach(func(txID utxo.TransactionID) {
			a.mutex.Lock(txID)
			defer a.mutex.Unlock(txID)

			if attachmentsOfTX := a.storage(txID, false); attachmentsOfTX != nil && attachmentsOfTX.Delete(epochIndex) && attachmentsOfTX.Size() == 0 {
				a.attachments.Delete(txID)
			}
		})
	}
}

func (a *attachments) storeAttachment(txID utxo.TransactionID, block *Block) {
	attachmentsOfEpoch, _ := a.storage(txID, true).RetrieveOrCreate(block.ID().EpochIndex, func() set.Set[*Block] {
		return set.New[*Block](true)
	})
	attachmentsOfEpoch.Add(block)
}

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks set.Set[*Block]) bool {
			blocks.ForEach(func(block *Block) {
				if lowestTime.After(block.IssuingTime()) || lowestTime.IsZero() {
					lowestTime = block.IssuingTime()
					attachment = block
				}
			})
			return true
		})
	}

	return
}

func (a *attachments) getLatestAttachment(txID utxo.TransactionID) (attachment *Block) {
	highestTime := time.Time{}
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks set.Set[*Block]) bool {
			blocks.ForEach(func(block *Block) {
				if highestTime.Before(block.IssuingTime()) {
					highestTime = block.IssuingTime()
					attachment = block
				}
			})
			return true
		})
	}

	return
}

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *memstorage.Storage[epoch.Index, set.Set[*Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.RetrieveOrCreate(txID, memstorage.New[epoch.Index, set.Set[*Block]])
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
