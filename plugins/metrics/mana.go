package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"go.uber.org/atomic"
)

var (
	accessMap           mana.NodeMap
	accessPercentile    atomic.Float64
	accessLock          sync.RWMutex
	consensusMap        mana.NodeMap
	consensusPercentile atomic.Float64
	consensusLock       sync.RWMutex
)

// AccessPercentile returns the top percentile the node belongs to in terms of access mana holders.
func AccessPercentile() float64 {
	return accessPercentile.Load()
}

// OwnAccessMana returns the access mana of the node.
func OwnAccessMana() float64 {
	accessLock.RLock()
	defer accessLock.RUnlock()
	return accessMap[local.GetInstance().ID()]
}

// AccessManaMap returns the access mana of the whole network.
func AccessManaMap() mana.NodeMap {
	accessLock.RLock()
	defer accessLock.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMap {
		result[k] = v
	}
	return result
}

// ConsensusPercentile returns the top percentile the node belongs to in terms of consensus mana holders.
func ConsensusPercentile() float64 {
	return consensusPercentile.Load()
}

// OwnConsensusMana returns the consensus mana of the node.
func OwnConsensusMana() float64 {
	consensusLock.RLock()
	defer consensusLock.RUnlock()
	return consensusMap[local.GetInstance().ID()]
}

// ConsensusManaMap returns the consensus mana of the whole network.
func ConsensusManaMap() mana.NodeMap {
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
	aPer, _ := accessMap.GetPercentile(local.GetInstance().ID())
	accessPercentile.Store(aPer)
	consensusLock.Lock()
	defer consensusLock.Unlock()
	consensusMap = tmp[mana.ConsensusMana]
	cPer, _ := consensusMap.GetPercentile(local.GetInstance().ID())
	consensusPercentile.Store(cPer)
}
