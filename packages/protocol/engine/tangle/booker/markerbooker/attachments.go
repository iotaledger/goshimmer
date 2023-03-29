package markerbooker

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

type attachments struct {
	attachments *shrinkingmap.ShrinkingMap[utxo.TransactionID, *shrinkingmap.ShrinkingMap[slot.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]]]
	evictionMap *shrinkingmap.ShrinkingMap[slot.Index, set.Set[utxo.TransactionID]]

	// nonOrphanedCounter is used to count all non-orphaned attachment of a transaction,
	// so that it's not necessary to iterate through all the attachments to check if the transaction is orphaned.
	nonOrphanedCounter *shrinkingmap.ShrinkingMap[utxo.TransactionID, uint32]

	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments:        shrinkingmap.New[utxo.TransactionID, *shrinkingmap.ShrinkingMap[slot.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]]](),
		evictionMap:        shrinkingmap.New[slot.Index, set.Set[utxo.TransactionID]](),
		nonOrphanedCounter: shrinkingmap.New[utxo.TransactionID, uint32](),

		mutex: syncutils.NewDAGMutex[utxo.TransactionID](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *booker.Block) (created bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	if !a.storeAttachment(txID, block) {
		return false
	}

	prevValue, _ := a.nonOrphanedCounter.GetOrCreate(txID, func() uint32 { return 0 })
	a.nonOrphanedCounter.Set(txID, prevValue+1)

	a.updateEvictionMap(block.ID().SlotIndex, txID)

	return true
}

func (a *attachments) AttachmentOrphaned(txID utxo.TransactionID, block *booker.Block) (attachmentBlock *booker.Block, attachmentOrphaned, lastAttachmentOrphaned bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	storage := a.storage(txID, false)
	if storage == nil {
		// zero attachments registered
		return nil, false, false
	}

	attachmentsOfSlot, exists := storage.Get(block.ID().Index())
	if !exists {
		// there is no attachments of the transaction in the slot, so no need to continue
		return nil, false, false
	}

	attachmentBlock, attachmentExists := attachmentsOfSlot.Get(block.ID())
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

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*booker.Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ slot.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *booker.Block) bool {
				attachments = append(attachments, block)
				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) GetAttachmentBlocks(txID utxo.TransactionID) (attachments *advancedset.AdvancedSet[*booker.Block]) {
	attachments = advancedset.New[*booker.Block]()

	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ slot.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, attachmentBlock *booker.Block) bool {
				attachments.Add(attachmentBlock)
				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) Evict(slotIndex slot.Index) {
	if txIDs, exists := a.evictionMap.Get(slotIndex); exists {
		a.evictionMap.Delete(slotIndex)

		txIDs.ForEach(func(txID utxo.TransactionID) {
			a.mutex.Lock(txID)
			defer a.mutex.Unlock(txID)

			if attachmentsOfTX := a.storage(txID, false); attachmentsOfTX != nil && attachmentsOfTX.Delete(slotIndex) && attachmentsOfTX.Size() == 0 {
				a.attachments.Delete(txID)
				a.nonOrphanedCounter.Delete(txID)
			}
		})
	}
}

func (a *attachments) storeAttachment(txID utxo.TransactionID, block *booker.Block) (created bool) {
	attachmentsOfSlot, _ := a.storage(txID, true).GetOrCreate(block.ID().SlotIndex, func() *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block] {
		return shrinkingmap.New[models.BlockID, *booker.Block]()
	})

	return lo.Return2(attachmentsOfSlot.GetOrCreate(block.ID(), func() *booker.Block {
		return block
	}))
}

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *booker.Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ slot.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *booker.Block) bool {
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

func (a *attachments) getLatestAttachment(txID utxo.TransactionID) (attachment *booker.Block) {
	highestTime := time.Time{}
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ slot.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *booker.Block) bool {
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

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *shrinkingmap.ShrinkingMap[slot.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.GetOrCreate(txID, func() *shrinkingmap.ShrinkingMap[slot.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]] {
			return shrinkingmap.New[slot.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *booker.Block]]()
		})
		return
	}

	storage, _ = a.attachments.Get(txID)

	return
}

func (a *attachments) updateEvictionMap(slotIndex slot.Index, txID utxo.TransactionID) {
	txIDs, _ := a.evictionMap.GetOrCreate(slotIndex, func() set.Set[utxo.TransactionID] {
		return set.New[utxo.TransactionID](true)
	})
	txIDs.Add(txID)
}
