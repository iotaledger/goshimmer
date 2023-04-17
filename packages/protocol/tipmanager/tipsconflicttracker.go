package tipmanager

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag"
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
	c.engine.Events.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflictID utxo.TransactionID) {
		c.deleteConflict(conflictID)
	}, event.WithWorkerPool(c.workerPool))
	c.engine.Events.Ledger.MemPool.ConflictDAG.ConflictRejected.Hook(func(conflictID utxo.TransactionID) {
		c.deleteConflict(conflictID)
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

		if !c.engine.Ledger.MemPool().ConflictDAG().AcceptanceState(advancedset.New(conflict)).IsPending() {
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

		if !c.engine.Ledger.MemPool().ConflictDAG().AcceptanceState(advancedset.New(conflictID)).IsPending() {
			continue
		}

		count--
		if count == 0 {
			c.censoredConflicts.Set(conflictID, types.Void)
			c.tipCountPerConflict.Delete(conflictID)
		}
	}
}

func (c *TipsConflictTracker) MissingConflicts(amount int, conflictDAG newconflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (missingConflicts utxo.TransactionIDs) {
	c.Lock()
	defer c.Unlock()

	missingConflicts = utxo.NewTransactionIDs()
	for _, conflictID := range c.censoredConflicts.Keys() {
		// TODO: this should not be necessary if ConflictAccepted/ConflictRejected events are fired appropriately
		// If the conflict is not pending anymore or it clashes with a conflict we already introduced, we can remove it from the censored conflicts.
		if !c.engine.Ledger.MemPool().ConflictDAG().AcceptanceState(advancedset.New(conflictID)).IsPending() {
			c.censoredConflicts.Delete(conflictID)
			continue
		}

		// We want to reintroduce only the pending conflict that is liked.
		if !conflictDAG.LikedInstead(advancedset.New(conflictID)).IsEmpty() {
			c.censoredConflicts.Delete(conflictID)

			continue
		}

		if missingConflicts.Add(conflictID) && missingConflicts.Size() == amount {
			return missingConflicts
		}
	}

	return missingConflicts
}

func (c *TipsConflictTracker) deleteConflict(id utxo.TransactionID) {
	c.Lock()
	defer c.Unlock()

	c.censoredConflicts.Delete(id)
	c.tipCountPerConflict.Delete(id)
}
