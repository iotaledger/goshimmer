package dashboard

import "sync"

type conflictRecord struct {
	conflictSet ConflictSet
	size        uint32
	buffer      []string
	lock        sync.RWMutex
}

func NewConflictRecord(recordSize uint32) *conflictRecord {
	return &conflictRecord{
		conflictSet: make(ConflictSet),
		size:        recordSize,
		buffer:      []string{},
	}
}

func (cr *conflictRecord) Update(ID string, c Conflict) {
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

	for nodeID, context := range c.NodesView {
		cr.conflictSet[ID].NodesView[nodeID] = context
	}
}
