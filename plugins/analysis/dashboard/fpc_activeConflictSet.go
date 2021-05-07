package dashboard

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
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

func (cr *activeConflictSet) load(id string) (conflict, bool) {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	// update the internal state
	if c, ok := cr.conflictSet[id]; !ok {
		return c, false
	}

	return cr.conflictSet[id], true
}

func (cr *activeConflictSet) update(id string, c conflict) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[id]; !ok {
		cr.conflictSet[id] = newConflict()
	}

	for nodeID, context := range c.NodesView {
		cr.conflictSet[id].NodesView[nodeID] = context
	}

	tmp := cr.conflictSet[id]
	tmp.Modified = clock.SyncedTime()
	cr.conflictSet[id] = tmp
}

func (cr *activeConflictSet) delete(id string) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[id]; !ok {
		return
	}

	delete(cr.conflictSet, id)
}
