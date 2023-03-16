package newconflictdag

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

type ConflictSet[ConflictID, ResourceID IDType] struct {
	conflicts *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	mutex sync.RWMutex
}

func (c *ConflictSet[ConflictID, ResourceID]) RegisterConflict(newConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.conflicts.Add(newConflict) {
		return
	}

	lighterConflicts := make([]*Conflict[ConflictID, ResourceID], 0)
	for _, existingConflict := range c.conflicts.Slice() {
		if comparison := existingConflict.CompareTo(newConflict); comparison == weight.Equal || comparison == weight.Heavier && newConflict.registerHeavierConflict(existingConflict) {
			continue
		}

		lighterConflicts = append(lighterConflicts, existingConflict)
	}

	for _, lighterConflict := range lighterConflicts {
		lighterConflict.registerHeavierConflict(newConflict)
	}
}
