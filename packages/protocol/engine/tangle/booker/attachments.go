package booker

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type attachments struct {
	attachments        *memstorage.Storage[utxo.TransactionID, *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *AttachmentBlock]]]
	evictionMap        *memstorage.Storage[epoch.Index, set.Set[utxo.TransactionID]]
	nonOrphanedCounter *memstorage.Storage[utxo.TransactionID, uint32]

	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments: memstorage.New[utxo.TransactionID, *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *AttachmentBlock]]](),
		evictionMap: memstorage.New[epoch.Index, set.Set[utxo.TransactionID]](),
		mutex:       syncutils.NewDAGMutex[utxo.TransactionID](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *Block) (attachmentBlock *AttachmentBlock, created bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	attachmentBlock, created = a.storeAttachment(txID, block)
	if !created {
		return nil, false
	}

	prevValue, _ := a.nonOrphanedCounter.RetrieveOrCreate(txID, func() uint32 { return 0 })
	a.nonOrphanedCounter.Set(txID, prevValue+1)

	a.updateEvictionMap(block.ID().EpochIndex, txID)

	return attachmentBlock, true
}

func (a *attachments) OrphanAttachment(txID utxo.TransactionID, block *Block) (attachmentBlock *AttachmentBlock, attachmentOrphaned, lastAttachmentOrphaned bool) {
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

	if !attachmentBlock.SetOrphaned(true) {
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

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *AttachmentBlock]) bool {
			blocks.ForEach(func(_ models.BlockID, attachmentBlock *AttachmentBlock) bool {
				attachments = append(attachments, attachmentBlock.Block)
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

func (a *attachments) storeAttachment(txID utxo.TransactionID, block *Block) (attachmentBlock *AttachmentBlock, created bool) {
	attachmentsOfEpoch, _ := a.storage(txID, true).RetrieveOrCreate(block.ID().EpochIndex, func() *memstorage.Storage[models.BlockID, *AttachmentBlock] {
		return memstorage.New[models.BlockID, *AttachmentBlock]()
	})

	return attachmentsOfEpoch.RetrieveOrCreate(block.ID(), func() *AttachmentBlock {
		return NewAttachmentBlock(block)
	})
}

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *AttachmentBlock]) bool {
			blocks.ForEach(func(_ models.BlockID, attachmentBlock *AttachmentBlock) bool {
				if lowestTime.After(attachmentBlock.IssuingTime()) || lowestTime.IsZero() {
					lowestTime = attachmentBlock.IssuingTime()
					attachment = attachmentBlock.Block
				}

				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) getLatestAttachment(txID utxo.TransactionID) (attachment *Block) {
	highestTime := time.Time{}
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *memstorage.Storage[models.BlockID, *AttachmentBlock]) bool {
			blocks.ForEach(func(_ models.BlockID, attachmentBlock *AttachmentBlock) bool {
				if highestTime.Before(attachmentBlock.IssuingTime()) {
					highestTime = attachmentBlock.IssuingTime()
					attachment = attachmentBlock.Block
				}
				return true
			})
			return true
		})
	}
	return
}

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *memstorage.Storage[epoch.Index, *memstorage.Storage[models.BlockID, *AttachmentBlock]]) {
	if createIfMissing {
		storage, _ = a.attachments.RetrieveOrCreate(txID, memstorage.New[epoch.Index, *memstorage.Storage[models.BlockID, *AttachmentBlock]])
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

// region AttachmentBlock //////////////////////////////////////////////////////////////////////////////////////////////

type AttachmentBlock struct {
	conflictCounted bool
	orphaned        bool

	*Block
}

func NewAttachmentBlock(block *Block) *AttachmentBlock {
	return &AttachmentBlock{Block: block}
}

func (a *AttachmentBlock) ConflictCounted() bool {
	a.RLock()
	defer a.RUnlock()

	return a.conflictCounted
}

func (a *AttachmentBlock) SetConflictCounted(conflictCounted bool) (updated bool) {
	a.Lock()
	defer a.Unlock()

	if a.conflictCounted == conflictCounted {
		return false
	}

	a.conflictCounted = conflictCounted
	return true
}

func (a *AttachmentBlock) Orphaned() bool {
	a.RLock()
	defer a.RUnlock()

	return a.orphaned
}

func (a *AttachmentBlock) SetOrphaned(orphaned bool) (updated bool) {
	a.Lock()
	defer a.Unlock()

	if a.orphaned == orphaned {
		return false
	}

	a.orphaned = orphaned
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
