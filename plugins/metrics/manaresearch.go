package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"go.uber.org/atomic"
)

var (
	// Mana 1 internal metrics
	// mapping nodeID -> mana value
	accessMapBM1 mana.NodeMap
	// mutex to protect the map
	accessLockBM1 sync.RWMutex
	// our own node's percentile in accessMapBM1
	accessPercentileBM1 atomic.Float64
	// mapping nodeID -> mana value
	consensusMapBM1 mana.NodeMap
	// mutex to protect the map
	consensusLockBm1 sync.RWMutex
	// our own node's percentile in consensusMapBM1
	consensusPercentileBM1 atomic.Float64

	// Mana 2 internal  metrics
	// mapping nodeID -> mana value
	accessMapBM2 mana.NodeMap
	// mutex to protect the map
	accessLockBM2 sync.RWMutex
	// our own node's percentile in accessMapBM2
	accessPercentileBM2 atomic.Float64
	// mapping nodeID -> mana value
	consensusMapBM2 mana.NodeMap
	// mutex to protect the map
	consensusLockBM2 sync.RWMutex
	// our own node's percentile in consensusMapBM2
	consensusPercentileBM2 atomic.Float64
)

//region Mana 1 exported metrics. Modules accessing Mana 1 metrics call these functions.

// AccessPercentileBM1 returns the top percentile the node belongs to in terms of access mana holders, only taking EBM1
// into account.
func AccessPercentileBM1() float64 {
	return accessPercentileBM1.Load()
}

// OwnAccessManaBM1 returns the access mana of the node, only taking EBM1 into account.
func OwnAccessManaBM1() float64 {
	accessLockBM1.RLock()
	defer accessLockBM1.RUnlock()
	return accessMapBM1[local.GetInstance().ID()]
}

// AccessManaMapBM1 returns the access mana of the whole network, only taking EBM1 into account.
func AccessManaMapBM1() mana.NodeMap {
	accessLockBM1.RLock()
	defer accessLockBM1.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMapBM1 {
		result[k] = v
	}
	return result
}

// ConsensusPercentileBM1 returns the top percentile the node belongs to in terms of consensus mana holders, only
// taking EBM1 into account.
func ConsensusPercentileBM1() float64 {
	return consensusPercentileBM1.Load()
}

// OwnConsensusManaBM1 returns the consensus mana of the node, only taking EBM1 into account.
func OwnConsensusManaBM1() float64 {
	consensusLockBm1.RLock()
	defer consensusLockBm1.RUnlock()
	return consensusMapBM1[local.GetInstance().ID()]
}

// ConsensusManaMapBM1 returns the consensus mana of the whole network, only taking EBM1 into account.
func ConsensusManaMapBM1() mana.NodeMap {
	consensusLockBm1.RLock()
	defer consensusLockBm1.RUnlock()
	result := mana.NodeMap{}
	for k, v := range consensusMapBM1 {
		result[k] = v
	}
	return result
}

//endregion

//region Mana 2 exported metrics. Modules accessing Mana 1 metrics call these functions.

// AccessPercentileBM2 returns the top percentile the node belongs to in terms of access mana holders, only taking EBM2
// into account.
func AccessPercentileBM2() float64 {
	return accessPercentileBM2.Load()
}

// OwnAccessManaBM2 returns the access mana of the node, only taking EBM2 into account.
func OwnAccessManaBM2() float64 {
	accessLockBM2.RLock()
	defer accessLockBM2.RUnlock()
	return accessMapBM2[local.GetInstance().ID()]
}

// AccessManaMapBM2 returns the access mana of the whole network, only taking EBM2 into account.
func AccessManaMapBM2() mana.NodeMap {
	accessLockBM2.RLock()
	defer accessLockBM2.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessMapBM2 {
		result[k] = v
	}
	return result
}

// ConsensusPercentileBM2 returns the top percentile the node belongs to in terms of consensus mana holders, only taking
// EBM2 into account.
func ConsensusPercentileBM2() float64 {
	return consensusPercentileBM2.Load()
}

// OwnConsensusManaBM2 returns the consensus mana of the node, only taking EBM2 into account.
func OwnConsensusManaBM2() float64 {
	consensusLockBM2.RLock()
	defer consensusLockBM2.RUnlock()
	return consensusMapBM2[local.GetInstance().ID()]
}

// ConsensusManaMapBM2 returns the consensus mana of the whole network, only taking EBM2 into account.
func ConsensusManaMapBM2() mana.NodeMap {
	consensusLockBM2.RLock()
	defer consensusLockBM2.RUnlock()
	result := mana.NodeMap{}
	for k, v := range consensusMapBM2 {
		result[k] = v
	}
	return result
}

//endregion

//region Functions for periodically updating internal metrics.

func measureManaBM1() {
	tmp := manaPlugin.GetAllManaMaps()
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

func measureManaBM2() {
	tmp := manaPlugin.GetAllManaMaps()
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

//endregion
