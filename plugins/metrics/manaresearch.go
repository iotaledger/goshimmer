package metrics

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/mana"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	// Access research mana (Mana1&Mana2 50-50)
	// mapping nodeID -> mana value
	accessResearchMap mana.NodeMap
	// mutex to protect the map
	accessResearchLock sync.RWMutex
	// our own node's percentile in accessResearchMap
	accessResearchPercentile atomic.Float64

	// Consensus research mana (Mana1&Mana2 50-50)
	// mapping nodeID -> mana value
	consensusResearchMap mana.NodeMap
	// mutex to protect the map
	consensusResearchLock sync.RWMutex
	// our own node's percentile in consensusResearchMap
	consensusResearchPercentile atomic.Float64
)

// region access research exported metrics. Modules accessing access research metrics call these functions.

// AccessResearchPercentile returns the top percentile the node belongs to in terms of access mana holders, only taking
// ResearchAccess mana into account.
func AccessResearchPercentile() float64 {
	return accessResearchPercentile.Load()
}

// OwnAccessResearchMana returns the access mana of the node, only taking ResearchAccess mana into account.
func OwnAccessResearchMana() float64 {
	accessResearchLock.RLock()
	defer accessResearchLock.RUnlock()
	return accessResearchMap[deps.Local.ID()]
}

// AccessResearchManaMap returns the access mana of the whole network, only taking ResearchAccess mana into account.
func AccessResearchManaMap() mana.NodeMap {
	accessResearchLock.RLock()
	defer accessResearchLock.RUnlock()
	result := mana.NodeMap{}
	for k, v := range accessResearchMap {
		result[k] = v
	}
	return result
}

// endregion

// region consensus research exported metrics. Modules accessing consensus research metrics call these functions.

// ConsensusResearchPercentile returns the top percentile the node belongs to in terms of consensus mana holders, only taking
// ResearchConsensus mana into account.
func ConsensusResearchPercentile() float64 {
	return consensusResearchPercentile.Load()
}

// OwnConsensusResearchMana returns the consensus mana of the node, only taking ResearchConsensus mana into account.
func OwnConsensusResearchMana() float64 {
	consensusResearchLock.RLock()
	defer consensusResearchLock.RUnlock()
	return consensusResearchMap[deps.Local.ID()]
}

// ConsensusResearchManaMap returns the consensus mana of the whole network, only taking ResearchConsensus mana into account.
func ConsensusResearchManaMap() mana.NodeMap {
	consensusResearchLock.RLock()
	defer consensusResearchLock.RUnlock()
	result := mana.NodeMap{}
	for k, v := range consensusResearchMap {
		result[k] = v
	}
	return result
}

// endregion

// region Functions for periodically updating internal metrics.

func measureAccessResearchMana() {
	accessResearchLock.Lock()
	defer accessResearchLock.Unlock()
	accessResearchMap, _, _ = manaPlugin.GetManaMap(mana.ResearchAccess)
	aPer, _ := accessResearchMap.GetPercentile(deps.Local.ID())
	accessResearchPercentile.Store(aPer)
}

func measureConsensusResearchMana() {
	consensusResearchLock.Lock()
	defer consensusResearchLock.Unlock()
	consensusResearchMap, _, _ = manaPlugin.GetManaMap(mana.ResearchConsensus)
	aPer, _ := consensusResearchMap.GetPercentile(deps.Local.ID())
	consensusResearchPercentile.Store(aPer)
}

// endregion
