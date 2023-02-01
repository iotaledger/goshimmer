package tipmanager

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"
)

type TipsConflictTracker struct {
	censoredConflicts   *memstorage.Storage[utxo.TransactionID, types.Empty]
	tipCountPerConflict *memstorage.Storage[utxo.TransactionID, int]
	engine              *engine.Engine

	sync.RWMutex
}

func NewTipsConflictTracker(engineInstance *engine.Engine) *TipsConflictTracker {
	return &TipsConflictTracker{
		censoredConflicts:   memstorage.New[utxo.TransactionID, types.Empty](),
		tipCountPerConflict: memstorage.New[utxo.TransactionID, int](),
		engine:              engineInstance,
	}
}

func (c *TipsConflictTracker) Setup() {
	c.engine.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(event.NewClosure(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}))
	c.engine.Ledger.ConflictDAG.Events.ConflictRejected.Attach(event.NewClosure(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}))
	c.engine.Ledger.ConflictDAG.Events.ConflictNotConflicting.Attach(event.NewClosure(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		c.deleteConflict(conflict.ID())
	}))
}

func (c *TipsConflictTracker) AddTip(block *scheduler.Block) {
	blockConflictIDs := c.engine.Tangle.Booker.BlockConflicts(block.Block.Block)

	c.Lock()
	defer c.Unlock()

	for it := blockConflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()

		if !c.engine.Ledger.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflict)).IsPending() {
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
	blockConflictIDs := c.engine.Tangle.Booker.BlockConflicts(block.Block.Block)

	c.Lock()
	defer c.Unlock()

	for it := blockConflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		count, _ := c.tipCountPerConflict.Get(conflictID)
		if count == 0 {
			continue
		}

		if !c.engine.Ledger.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflictID)).IsPending() {
			continue
		}

		if count--; count == 0 {
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
	c.censoredConflicts.ForEach(func(conflictID utxo.TransactionID, _ types.Empty) bool {
		// TODO: this should not be necessary if ConflictAccepted/ConflictRejected events are fired appropriately
		if !c.engine.Ledger.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflictID)).IsPending() {
			censoredConflictsToDelete.Add(conflictID)
			return true
		}

		// We want to reintroduce only the pending conflict that is liked
		conflict, exists := c.engine.Ledger.ConflictDAG.Conflict(conflictID)
		if !exists {
			return true
		}

		if !c.engine.Consensus.ConflictLiked(conflict) {
			return true
		}

		fmt.Println(">> Missing conflict rescued", conflictID)
		if missingConflicts.Add(conflictID) && missingConflicts.Size() == amount {
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
