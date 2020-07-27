package dashboard

import (
	"sync"
	"time"
)

// activeConflictSet contains the set of the active conflicts, not yet finalized.
type activeConflictSet struct {
	conflictSet conflictSet
	lock        sync.RWMutex
}

func newActiveConflictSet() *activeConflictSet {
	return &activeConflictSet{
		conflictSet: make(conflictSet),
	}
}

// cleanUp removes the finalized conflicts from the active conflicts set.
func (cr *activeConflictSet) cleanUp() {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	for id, conflict := range cr.conflictSet {
		if conflict.isFinalized() || conflict.isOlderThan(1*time.Minute) {
			delete(cr.conflictSet, id)
		}
	}
}

func (cr *activeConflictSet) ToFPCUpdate() *FPCUpdate {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	return &FPCUpdate{
		Conflicts: cr.conflictSet,
	}
}

func (cr *activeConflictSet) load(ID string) (conflict, bool) {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	// update the internal state
	if c, ok := cr.conflictSet[ID]; !ok {
		return c, false
	}

	return cr.conflictSet[ID], true
}

func (cr *activeConflictSet) update(ID string, c conflict) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[ID]; !ok {
		cr.conflictSet[ID] = newConflict()
	}

	for nodeID, context := range c.NodesView {
		cr.conflictSet[ID].NodesView[nodeID] = context
	}

	tmp := cr.conflictSet[ID]
	tmp.Modified = time.Now()
	cr.conflictSet[ID] = tmp
}

func (cr *activeConflictSet) delete(ID string) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[ID]; !ok {
		return
	}

	delete(cr.conflictSet, ID)
}
