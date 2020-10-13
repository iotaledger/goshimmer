package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"go.uber.org/atomic"
)

var (
	accessMapBM1           mana.NodeMap
	accessPercentileBM1    atomic.Float64
	accessLockBM1          sync.RWMutex
	consensusMapBM1        mana.NodeMap
	consensusPercentileBM1 atomic.Float64
	consensusLockBm1       sync.RWMutex
)

// AccessPercentileBM1 returns the top percentile the node belongs to in terms of access mana holders.
func AccessPercentileBM1() float64 {
	return accessPercentileBM1.Load()
}

// OwnAccessManaBM1 returns the access mana of the node.
func OwnAccessManaBM1() float64 {
	accessLockBM1.RLock()
	defer accessLockBM1.RUnlock()
	return accessMapBM1[local.GetInstance().ID()]
}

// AccessManaMapBM1 returns the access mana of the whole network.
func AccessManaMapBM1() mana.NodeMap {
	accessLockBM1.RLock()
	defer accessLockBM1.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMapBM1 {
		result[k] = v
	}
	return result
}

// ConsensusPercentileBM1 returns the top percentile the node belongs to in terms of consensus mana holders.
func ConsensusPercentileBM1() float64 {
	return consensusPercentileBM1.Load()
}

// OwnConsensusManaBM1 returns the consensus mana of the node.
func OwnConsensusManaBM1() float64 {
	consensusLockBm1.RLock()
	defer consensusLockBm1.RUnlock()
	return consensusMapBM1[local.GetInstance().ID()]
}

// ConsensusManaMapBM1 returns the consensus mana of the whole network.
func ConsensusManaMapBM1() mana.NodeMap {
	consensusLockBm1.RLock()
	defer consensusLockBm1.RUnlock()
	result := mana.NodeMap{}
	for k, v := range consensusMapBM1 {
		result[k] = v
	}
	return result
}

func measureManaBM1() {
	tmp := manaPlugin.GetAllManaMaps(mana.OnlyMana1)
	accessLockBM1.Lock()
	defer accessLockBM1.Unlock()
	accessMapBM1 = tmp[mana.AccessMana]
	aPer, _ := accessMapBM1.GetPercentile(local.GetInstance().ID())
	accessPercentileBM1.Store(aPer)
	consensusLockBm1.Lock()
	defer consensusLockBm1.Unlock()
	consensusMapBM1 = tmp[mana.ConsensusMana]
	cPer, _ := consensusMapBM1.GetPercentile(local.GetInstance().ID())
	consensusPercentileBM1.Store(cPer)
}

var (
	accessMapBM2           mana.NodeMap
	accessPercentileBM2    atomic.Float64
	accessLockBM2          sync.RWMutex
	consensusMapBM2        mana.NodeMap
	consensusPercentileBM2 atomic.Float64
	consensusLockBM2       sync.RWMutex
)

// AccessPercentileBM2 returns the top percentile the node belongs to in terms of access mana holders.
func AccessPercentileBM2() float64 {
	return accessPercentileBM2.Load()
}

// OwnAccessManaBM2 returns the access mana of the node.
func OwnAccessManaBM2() float64 {
	accessLockBM2.RLock()
	defer accessLockBM2.RUnlock()
	return accessMapBM2[local.GetInstance().ID()]
}

// AccessManaMapBM2 returns the access mana of the whole network.
func AccessManaMapBM2() mana.NodeMap {
	accessLockBM2.RLock()
	defer accessLockBM2.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMapBM2 {
		result[k] = v
	}
	return result
}

// ConsensusPercentileBM2 returns the top percentile the node belongs to in terms of consensus mana holders.
func ConsensusPercentileBM2() float64 {
	return consensusPercentileBM2.Load()
}

// OwnConsensusManaBM2 returns the consensus mana of the node.
func OwnConsensusManaBM2() float64 {
	consensusLockBM2.RLock()
	defer consensusLockBM2.RUnlock()
	return consensusMapBM2[local.GetInstance().ID()]
}

// ConsensusManaMapBM2 returns the consensus mana of the whole network.
func ConsensusManaMapBM2() mana.NodeMap {
	consensusLockBM2.RLock()
	defer consensusLockBM2.RUnlock()
	result := mana.NodeMap{}
	for k, v := range consensusMapBM2 {
		result[k] = v
	}
	return result
}

func measureManaBM2() {
	tmp := manaPlugin.GetAllManaMaps(mana.OnlyMana2)
	accessLockBM2.Lock()
	defer accessLockBM2.Unlock()
	accessMapBM2 = tmp[mana.AccessMana]
	aPer, _ := accessMapBM2.GetPercentile(local.GetInstance().ID())
	accessPercentileBM2.Store(aPer)
	consensusLockBM2.Lock()
	defer consensusLockBM2.Unlock()
	consensusMapBM2 = tmp[mana.ConsensusMana]
	cPer, _ := consensusMapBM2.GetPercentile(local.GetInstance().ID())
	consensusPercentileBM2.Store(cPer)
}
