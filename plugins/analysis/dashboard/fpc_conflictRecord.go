package dashboard

import "sync"

type conflictRecord struct {
	conflictSet conflictSet
	size        uint32
	buffer      []string
	lock        sync.RWMutex
}

func newConflictRecord(recordSize uint32) *conflictRecord {
	return &conflictRecord{
		conflictSet: make(conflictSet),
		size:        recordSize,
		buffer:      []string{},
	}
}

func (cr *conflictRecord) ToFPCUpdate() *FPCUpdate {
	cr.lock.RLock()
	defer cr.lock.RUnlock()

	// log.Info("FPC refresh conflicts: ", len(cr.conflictSet))
	// for k, v := range cr.conflictSet {
	// 	log.Info("Conflict ID: ", k, len(v.NodesView))
	// }

	return &FPCUpdate{
		conflicts: cr.conflictSet,
	}
}

func (cr *conflictRecord) update(ID string, c conflict) {
	lock.Lock()
	defer lock.Unlock()

	// update the internal state
	if _, ok := cr.conflictSet[ID]; !ok {
		cr.conflictSet[ID] = newConflict()

		cr.buffer = append(cr.buffer, ID)
		if len(cr.buffer) > int(cr.size) {
			delete(cr.conflictSet, cr.buffer[0])
			cr.buffer = cr.buffer[1:]
		}
	}

	for nodeID, context := range c.nodesView {
		cr.conflictSet[ID].nodesView[nodeID] = context
	}
}
