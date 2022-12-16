package latestblocktracker

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// LatestBlockTracker is a component that tracks ID of the latest seen Block.
type LatestBlockTracker struct {
	blockID models.BlockID
	time    time.Time

	m sync.RWMutex
}

// New return a new LatestBlockTracker.
func New() (newLatestBlockTracker *LatestBlockTracker) {
	return &LatestBlockTracker{}
}

// Update updates the latest seen Block.
func (t *LatestBlockTracker) Update(block *models.Block) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.time.After(block.IssuingTime()) {
		return
	}

	t.blockID = block.ID()
	t.time = block.IssuingTime()
}

// BlockID returns the ID of the latest seen Block.
func (t *LatestBlockTracker) BlockID() (blockID models.BlockID) {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.blockID
}
