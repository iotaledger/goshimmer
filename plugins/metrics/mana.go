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
	defer accessLock.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMap {
		result[k] = v
	}
	return result
}

// ConsensusMana returns the consensus mana of the whole network.
func ConsensusMana() mana.NodeMap {
	consensusLock.RLock()
	defer consensusLock.RUnlock()
	result := mana.NodeMap{}
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
