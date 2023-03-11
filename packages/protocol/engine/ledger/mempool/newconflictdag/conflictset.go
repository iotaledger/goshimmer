package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
)

type ConflictSet[ConflictIDType, ResourceIDType comparable] struct {
	conflicts *advancedset.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]

	mutex sync.RWMutex
}

func (c *ConflictSet[ConflictIDType, ResourceIDType]) addConflict(conflict *Conflict[ConflictIDType, ResourceIDType]) (added bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if added = c.conflicts.Add(conflict); added {
		c.updatedConflictsWithHigherWeight(conflict)
	}

	return added
}

func (c *ConflictSet[ConflictIDType, ResourceIDType]) updatedConflictsWithHigherWeight(conflict *Conflict[ConflictIDType, ResourceIDType]) {
	existingConflictsWithHigherWeight := make([]*Conflict[ConflictIDType, ResourceIDType], 0)
	_ = c.conflicts.ForEach(func(existingConflict *Conflict[ConflictIDType, ResourceIDType]) error {
		if conflict.weight.Compare(existingConflict.weight) == 1 {
			existingConflict.addConflictsWithHigherWeight(conflict)
		} else if conflict.weight.Compare(existingConflict.weight) == -1 {
			existingConflictsWithHigherWeight = append(existingConflictsWithHigherWeight, existingConflict)
		}

		return nil
	})

	conflict.addConflictsWithHigherWeight(existingConflictsWithHigherWeight...)
}
