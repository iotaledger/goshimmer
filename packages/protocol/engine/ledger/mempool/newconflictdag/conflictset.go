package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
)

type ConflictSet[ConflictID, ResourceID IDType] struct {
	conflicts *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	mutex sync.RWMutex
}

func (c *ConflictSet[ConflictID, ResourceID]) RegisterConflict(newConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.conflicts.Add(newConflict)
}
