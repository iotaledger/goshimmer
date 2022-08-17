package eviction

import "sync"

type State struct {
	*Manager

	sync.RWMutex
}

func NewState(manager *Manager) *State {
	return &State{
		Manager: manager,
	}
}
