package dashboard

import "sync"

type conflictRecord struct {
	conflictSet conflictSet
	lock        sync.RWMutex
}

func newConflictRecord() *conflictRecord {
	return &conflictRecord{
		conflictSet: make(conflictSet),
	}
}

func (cr *conflictRecord) cleanUp() {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	for id, conflict := range cr.conflictSet {
		if conflict.isFinalized() {
			delete(cr.conflictSet, id)
		}
	}
}

func (cr *conflictRecord) ToFPCUpdate() *FPCUpdate {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	return &FPCUpdate{
		Conflicts: cr.conflictSet,
	}
}

func (cr *conflictRecord) load(ID string) (conflict, bool) {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	// update the internal state
	if c, ok := cr.conflictSet[ID]; !ok {
		return c, false
	}

	return cr.conflictSet[ID], true
}

func (cr *conflictRecord) update(ID string, c conflict) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[ID]; !ok {
		cr.conflictSet[ID] = newConflict()
	}

	for nodeID, context := range c.NodesView {
		cr.conflictSet[ID].NodesView[nodeID] = context
	}
}

func (cr *conflictRecord) delete(ID string) {
	cr.lock.Lock()
	defer cr.lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[ID]; !ok {
		return
	}

	delete(cr.conflictSet, ID)
}
