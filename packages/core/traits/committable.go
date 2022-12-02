package traits

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Committable interface {
	SetLastCommittedEpoch(index epoch.Index)
	LastCommittedEpoch() (index epoch.Index)
}

func NewCommittable() Committable {
	return &committable{}
}

type committable struct {
	lastCommittedEpoch epoch.Index
	mutex              sync.RWMutex
}

func (u *committable) SetLastCommittedEpoch(index epoch.Index) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.lastCommittedEpoch = index
}

func (u *committable) LastCommittedEpoch() epoch.Index {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.lastCommittedEpoch
}
