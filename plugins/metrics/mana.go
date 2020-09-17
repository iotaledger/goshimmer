package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
)

var (
	accessMap     mana.NodeMap
	accessLock    sync.RWMutex
	consensusMap  mana.NodeMap
	consensusLock sync.RWMutex
)

// AccessMana returns the access mana of the whole network.
func AccessMana() mana.NodeMap {
	accessLock.RLock()
	accessLock.RUnlock()
	result := mana.NodeMap{}
	accessLock.RLock()
	for k, v := range accessMap {
		result[k] = v
	}
	return result
}

// ConsensusMana returns the consensus mana of the whole network.
func ConsensusMana() mana.NodeMap {
	consensusLock.RLock()
	consensusLock.RUnlock()
	result := mana.NodeMap{}
	accessLock.RLock()
	for k, v := range consensusMap {
		result[k] = v
	}
	return result
}

func measureMana() {
	tmp := manaPlugin.GetAllManaMaps()
	accessLock.Lock()
	defer accessLock.Unlock()
	accessMap = tmp[mana.AccessMana]
	consensusLock.Lock()
	defer consensusLock.Unlock()
	consensusMap = tmp[mana.ConsensusMana]
}
