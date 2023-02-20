package booker

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/syncutils"
)

type attachments struct {
	attachments *memstorage.Storage[utxo.TransactionID, *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *virtualvoting.Block]]]
	evictionMap *memstorage.Storage[epoch.Index, set.Set[utxo.TransactionID]]

	// nonOrphanedCounter is used to count all non-orphaned attachment of a transaction,
	// so that it's not necessary to iterate through all the attachments to check if the transaction is orphaned.
	nonOrphanedCounter *memstorage.Storage[utxo.TransactionID, uint32]

	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments:        memstorage.New[utxo.TransactionID, *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *virtualvoting.Block]]](),
		evictionMap:        memstorage.New[epoch.Index, set.Set[utxo.TransactionID]](),
		nonOrphanedCounter: memstorage.New[utxo.TransactionID, uint32](),

		mutex: syncutils.NewDAGMutex[utxo.TransactionID](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *virtualvoting.Block) (created bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	if !a.storeAttachment(txID, block) {
		return false
	}

	prevValue, _ := a.nonOrphanedCounter.RetrieveOrCreate(txID, func() uint32 { return 0 })
	a.nonOrphanedCounter.Set(txID, prevValue+1)

	a.updateEvictionMap(block.ID().EpochIndex, txID)

	return true
}

func (a *attachments) AttachmentOrphaned(txID utxo.TransactionID, block *virtualvoting.Block) (attachmentBlock *virtualvoting.Block, attachmentOrphaned, lastAttachmentOrphaned bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	storage := a.storage(txID, false)
	if storage == nil {
		// zero attachments registered
		return nil, false, false
	}

	attachmentsOfEpoch, exists := storage.Get(block.ID().Index())
	if !exists {
		// there is no attachments of the transaction in the epoch, so no need to continue
		return nil, false, false
	}

	attachmentBlock, attachmentExists := attachmentsOfEpoch.Get(block.ID())
	if !attachmentExists {
		return nil, false, false
	}

	prevValue, counterExists := a.nonOrphanedCounter.Get(txID)
	if !counterExists {
		panic(fmt.Sprintf("non orphaned attachment counter does not exist for TxID(%s)", txID.String()))
	}

	newValue := prevValue - 1
	a.nonOrphanedCounter.Set(txID, newValue)

	return attachmentBlock, true, newValue == 0
}

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*virtualvoting.Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *virtualvoting.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *virtualvoting.Block) bool {
				attachments = append(attachments, block)
				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) GetAttachmentBlocks(txID utxo.TransactionID) (attachments *set.AdvancedSet[*virtualvoting.Block]) {
	attachments = set.NewAdvancedSet[*virtualvoting.Block]()

	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *virtualvoting.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, attachmentBlock *virtualvoting.Block) bool {
				attachments.Add(attachmentBlock)
				return true
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

func (a *attachments) storeAttachment(txID utxo.TransactionID, block *virtualvoting.Block) (created bool) {
	attachmentsOfEpoch, _ := a.storage(txID, true).RetrieveOrCreate(block.ID().EpochIndex, func() *memstorage.Storage[models.BlockID, *virtualvoting.Block] {
		return memstorage.New[models.BlockID, *virtualvoting.Block]()
	})

	return lo.Return2(attachmentsOfEpoch.RetrieveOrCreate(block.ID(), func() *virtualvoting.Block {
		return block
	}))
}

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *virtualvoting.Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *virtualvoting.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *virtualvoting.Block) bool {
				if lowestTime.After(block.IssuingTime()) || lowestTime.IsZero() {
					lowestTime = block.IssuingTime()
					attachment = block
				}

				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) getLatestAttachment(txID utxo.TransactionID) (attachment *virtualvoting.Block) {
	highestTime := time.Time{}
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *virtualvoting.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *virtualvoting.Block) bool {
				if highestTime.Before(block.IssuingTime()) {
					highestTime = block.IssuingTime()
					attachment = block
				}
				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *virtualvoting.Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.RetrieveOrCreate(txID, memstorage.New[epoch.Index, *memstorage.Storage[models.BlockID, *virtualvoting.Block]])
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
