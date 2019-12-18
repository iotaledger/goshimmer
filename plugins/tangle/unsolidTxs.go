package tangle

import (
	"sync"
	"time"
)

type UnsolidTxs struct {
	internal map[string]Info
	sync.RWMutex
}

type Info struct {
	lastRequest time.Time
	counter     int
}

func NewUnsolidTxs() *UnsolidTxs {
	return &UnsolidTxs{
		internal: make(map[string]Info),
	}
}

func (u *UnsolidTxs) Add(hash string) {
	u.Lock()
	info := Info{
		lastRequest: time.Now(),
		counter:     1,
	}
	u.internal[hash] = info
	u.Unlock()
}

func (u *UnsolidTxs) Remove(hash string) {
	u.Lock()
	if _, exists := u.internal[hash]; !exists {
		delete(u.internal, hash)
	}
	u.Unlock()
}

func (u *UnsolidTxs) Update(targetTime time.Time) (result []string) {
	u.Lock()
	for k, v := range u.internal {
		if v.lastRequest.Before(targetTime) {
			result = append(result, k)

			v.lastRequest = time.Now()
			v.counter++

			u.internal[k] = v
		}
	}
	u.Unlock()
	return result
}
