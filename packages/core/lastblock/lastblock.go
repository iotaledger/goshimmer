package lastblock

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Tracker struct {
	blockID models.BlockID
	time time.Time

	m sync.RWMutex
}

func NewTracker() *Tracker {
	return &Tracker{}
}

func (t *Tracker) Update(block *models.Block) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.time.After(block.IssuingTime()) {
		return
	}

	t.blockID = block.ID()
	t.time = block.IssuingTime()
}

func (t *Tracker) BlockID() (blockID models.BlockID) {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.blockID
}
