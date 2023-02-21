package booker

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

type attachments struct {
	attachments *shrinkingmap.ShrinkingMap[utxo.TransactionID, *shrinkingmap.ShrinkingMap[epoch.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]]]
	evictionMap *shrinkingmap.ShrinkingMap[epoch.Index, set.Set[utxo.TransactionID]]

	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

func newAttachments() (newAttachments *attachments) {
	return &attachments{
		attachments: shrinkingmap.New[utxo.TransactionID, *shrinkingmap.ShrinkingMap[epoch.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]]](),
		evictionMap: shrinkingmap.New[epoch.Index, set.Set[utxo.TransactionID]](),

		mutex: syncutils.NewDAGMutex[utxo.TransactionID](),
	}
}

func (a *attachments) Store(txID utxo.TransactionID, block *virtualvoting.Block) (created bool) {
	a.mutex.Lock(txID)
	defer a.mutex.Unlock(txID)

	if !a.storeAttachment(txID, block) {
		return false
	}

	a.updateEvictionMap(block.ID().Index(), txID)

	return true
}

func (a *attachments) Get(txID utxo.TransactionID) (attachments []*virtualvoting.Block) {
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]) bool {
			blocks.ForEach(func(_ models.BlockID, block *virtualvoting.Block) bool {
				attachments = append(attachments, block)
				return true
			})
			return true
		})
	}

	return
}

func (a *attachments) GetAttachmentBlocks(txID utxo.TransactionID) (attachments *advancedset.AdvancedSet[*virtualvoting.Block]) {
	attachments = advancedset.NewAdvancedSet[*virtualvoting.Block]()

	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]) bool {
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
	attachmentsOfEpoch, _ := a.storage(txID, true).GetOrCreate(block.ID().EpochIndex, func() *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block] {
		return shrinkingmap.New[models.BlockID, *virtualvoting.Block]()
	})

	return lo.Return2(attachmentsOfEpoch.GetOrCreate(block.ID(), func() *virtualvoting.Block {
		return block
	}))
}

func (a *attachments) getEarliestAttachment(txID utxo.TransactionID) (attachment *virtualvoting.Block) {
	var lowestTime time.Time
	if txStorage := a.storage(txID, false); txStorage != nil {
		txStorage.ForEach(func(_ epoch.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]) bool {
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
		txStorage.ForEach(func(_ epoch.Index, blocks *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]) bool {
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

func (a *attachments) storage(txID utxo.TransactionID, createIfMissing bool) (storage *shrinkingmap.ShrinkingMap[epoch.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]]) {
	if createIfMissing {
		storage, _ = a.attachments.GetOrCreate(txID, func() *shrinkingmap.ShrinkingMap[epoch.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]] {
			return shrinkingmap.New[epoch.Index, *shrinkingmap.ShrinkingMap[models.BlockID, *virtualvoting.Block]]()
		})
		return
	}

	storage, _ = a.attachments.Get(txID)

	return
}

func (a *attachments) updateEvictionMap(epochIndex epoch.Index, txID utxo.TransactionID) {
	txIDs, _ := a.evictionMap.GetOrCreate(epochIndex, func() set.Set[utxo.TransactionID] {
		return set.New[utxo.TransactionID](true)
	})
	txIDs.Add(txID)
}
