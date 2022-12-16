package latestblocktracker

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// LatestBlockTracker is a component that tracks the ID of the latest Block.
type LatestBlockTracker struct {
	blockID models.BlockID
	time    time.Time
	mutex   sync.RWMutex
}

// New return a new LatestBlockTracker.
func New() (newLatestBlockTracker *LatestBlockTracker) {
	return new(LatestBlockTracker)
}

// Update updates the latest seen Block.
func (t *LatestBlockTracker) Update(block *models.Block) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.time.After(block.IssuingTime()) {
		return
	}

	t.blockID = block.ID()
	t.time = block.IssuingTime()
}

// BlockID returns the ID of the latest seen Block.
func (t *LatestBlockTracker) BlockID() (blockID models.BlockID) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.blockID
}
