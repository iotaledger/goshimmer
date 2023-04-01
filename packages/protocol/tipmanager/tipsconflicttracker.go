package tipmanager

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type TipsConflictTracker struct {
	workerPool          *workerpool.WorkerPool
	tipConflicts        *shrinkingmap.ShrinkingMap[models.BlockID, utxo.TransactionIDs]
	censoredConflicts   *shrinkingmap.ShrinkingMap[utxo.TransactionID, types.Empty]
	tipCountPerConflict *shrinkingmap.ShrinkingMap[utxo.TransactionID, int]
	engine              *engine.Engine

	sync.RWMutex
}

func NewTipsConflictTracker(workerPool *workerpool.WorkerPool, engineInstance *engine.Engine) *TipsConflictTracker {
	t := &TipsConflictTracker{
		workerPool:          workerPool,
		tipConflicts:        shrinkingmap.New[models.BlockID, utxo.TransactionIDs](),
		censoredConflicts:   shrinkingmap.New[utxo.TransactionID, types.Empty](),
		tipCountPerConflict: shrinkingmap.New[utxo.TransactionID, int](),
		engine:              engineInstance,
	}

	t.setup()
	return t
}

func (c *TipsConflictTracker) setup() {
	c.engine.Events.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}, event.WithWorkerPool(c.workerPool))
	c.engine.Events.Ledger.MemPool.ConflictDAG.ConflictRejected.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}, event.WithWorkerPool(c.workerPool))
	c.engine.Events.Ledger.MemPool.ConflictDAG.ConflictNotConflicting.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}, event.WithWorkerPool(c.workerPool))
}

func (c *TipsConflictTracker) Cleanup() {
	c.workerPool.Shutdown()
}

func (c *TipsConflictTracker) AddTip(block *scheduler.Block, blockConflictIDs utxo.TransactionIDs) {
	c.Lock()
	defer c.Unlock()

	c.tipConflicts.Set(block.ID(), blockConflictIDs)

	for it := blockConflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()

		if !c.engine.Ledger.MemPool().ConflictDAG().ConfirmationState(advancedset.New(conflict)).IsPending() {
			continue
		}

		count, _ := c.tipCountPerConflict.Get(conflict)
		c.tipCountPerConflict.Set(conflict, count+1)

		// If the conflict has now a tip representation, we can remove it from the censored conflicts.
		if count == 0 {
			c.censoredConflicts.Delete(conflict)
		}
	}
}

func (c *TipsConflictTracker) RemoveTip(block *scheduler.Block) {
	c.Lock()
	defer c.Unlock()

	blockConflictIDs, exists := c.tipConflicts.Get(block.ID())
	if !exists {
		return
	}

	c.tipConflicts.Delete(block.ID())

	for it := blockConflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		count, _ := c.tipCountPerConflict.Get(conflictID)
		if count == 0 {
			continue
		}

		if !c.engine.Ledger.MemPool().ConflictDAG().ConfirmationState(advancedset.New(conflictID)).IsPending() {
			continue
		}

		count--
		if count == 0 {
			c.censoredConflicts.Set(conflictID, types.Void)
			c.tipCountPerConflict.Delete(conflictID)
		}
	}
}

func (c *TipsConflictTracker) MissingConflicts(amount int) (missingConflicts utxo.TransactionIDs) {
	c.Lock()
	defer c.Unlock()

	missingConflicts = utxo.NewTransactionIDs()
	censoredConflictsToDelete := utxo.NewTransactionIDs()
	dislikedConflicts := utxo.NewTransactionIDs()
	c.censoredConflicts.ForEach(func(conflictID utxo.TransactionID, _ types.Empty) bool {
		// TODO: this should not be necessary if ConflictAccepted/ConflictRejected events are fired appropriately
		// If the conflict is not pending anymore or it clashes with a conflict we already introduced, we can remove it from the censored conflicts.
		if !c.engine.Ledger.MemPool().ConflictDAG().ConfirmationState(advancedset.New(conflictID)).IsPending() || dislikedConflicts.Has(conflictID) {
			censoredConflictsToDelete.Add(conflictID)
			return true
		}

		// We want to reintroduce only the pending conflict that is liked.
		likedConflictID, dislikedConflictsInner := c.engine.Consensus.ConflictResolver().AdjustOpinion(conflictID)
		dislikedConflicts.AddAll(dislikedConflictsInner)

		if missingConflicts.Add(likedConflictID) && missingConflicts.Size() == amount {
			// We stop iterating if we have enough conflicts
			return false
		}

		return true
	})

	for it := censoredConflictsToDelete.Iterator(); it.HasNext(); {
		conflictID := it.Next()
		c.censoredConflicts.Delete(conflictID)
		c.tipCountPerConflict.Delete(conflictID)
	}

	return missingConflicts
}

func (c *TipsConflictTracker) deleteConflict(id utxo.TransactionID) {
	c.Lock()
	defer c.Unlock()

	c.censoredConflicts.Delete(id)
	c.tipCountPerConflict.Delete(id)
}
