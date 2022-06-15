package tangle

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

type TipsConflictTracker struct {
	missingConflicts  *set.AdvancedSet[utxo.TransactionID]
	tipsConflictCount map[utxo.TransactionID]int
	tangle            *Tangle

	sync.RWMutex
}

func NewTipsConflictTracker(tangle *Tangle) *TipsConflictTracker {
	return &TipsConflictTracker{
		missingConflicts:  set.NewAdvancedSet[utxo.TransactionID](),
		tipsConflictCount: make(map[utxo.TransactionID]int),
		tangle:            tangle,
	}
}

func (c *TipsConflictTracker) Setup() {
	c.tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		c.deleteConflict(event.BranchID)
	}))
	c.tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		c.deleteConflict(event.BranchID)
	}))
}

func (c *TipsConflictTracker) AddTip(messageID MessageID) {
	messageConflictIDs, err := c.tangle.Booker.MessageBranchIDs(messageID)
	if err != nil {
		panic(err)
	}

	c.Lock()
	defer c.Unlock()

	for it := messageConflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if c.tangle.Ledger.ConflictDAG.InclusionState(set.NewAdvancedSet(conflictID)) != conflictdag.Pending {
			continue
		}

		if c.tipsConflictCount[conflictID]++; c.tipsConflictCount[conflictID] == 1 {
			c.missingConflicts.Delete(conflictID)
		}
	}
}

func (c *TipsConflictTracker) RemoveTip(messageID MessageID) {
	messageBranchIDs, err := c.tangle.Booker.MessageBranchIDs(messageID)
	if err != nil {
		panic("could not determine BranchIDs of tip.")
	}

	c.Lock()
	defer c.Unlock()

	for it := messageBranchIDs.Iterator(); it.HasNext(); {
		messageBranchID := it.Next()

		if _, exists := c.tipsConflictCount[messageBranchID]; exists {
			if c.tipsConflictCount[messageBranchID]--; c.tipsConflictCount[messageBranchID] == 0 {
				fmt.Println()

				if c.tangle.OTVConsensusManager != nil && c.tangle.OTVConsensusManager.BranchLiked(messageBranchID) {
					c.missingConflicts.Add(messageBranchID)
				}

				delete(c.tipsConflictCount, messageBranchID)
			}
		}
	}
}

func (c *TipsConflictTracker) MissingConflicts(amount int) (missingConflicts utxo.TransactionIDs) {
	c.RLock()
	defer c.RUnlock()

	missingConflicts = utxo.NewTransactionIDs()
	_ = c.missingConflicts.ForEach(func(conflictID utxo.TransactionID) (err error) {
		if c.tangle.OTVConsensusManager.BranchLiked(conflictID) && missingConflicts.Add(conflictID) && missingConflicts.Size() == amount {
			return errors.Errorf("amount of missing conflicts reached %d", amount)
		}

		return nil
	})

	return missingConflicts
}

func (c *TipsConflictTracker) deleteConflict(id utxo.TransactionID) {
	c.Lock()
	defer c.Unlock()

	c.missingConflicts.Delete(id)
	delete(c.tipsConflictCount, id)
}
