package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

// NodePledgeMap is a map of node and a list of mana pledges.
type NodePledgeMap map[identity.ID][]float64

var (
	accessMap                 mana.NodeMap
	accessPercentile          atomic.Float64
	accessLock                sync.RWMutex
	consensusMap              mana.NodeMap
	consensusPercentile       atomic.Float64
	consensusLock             sync.RWMutex
	accessPledgeLock          sync.RWMutex
	accessPledge              = NodePledgeMap{}
	consensusPledgeLock       sync.RWMutex
	consensusPledge           = NodePledgeMap{}
	averageNeighborsAccess    atomic.Float64
	averageNeighborsConsensus atomic.Float64
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

// AverageNeighborsAccess returns the average access mana of the nodes neighbors.
func AverageNeighborsAccess() float64 {
	return averageNeighborsAccess.Load()
}

// AverageNeighborsConsensus returns the average consensus mana of the nodes neighbors.
func AverageNeighborsConsensus() float64 {
	return averageNeighborsConsensus.Load()
}

// AveragePledgeAccessMap returns the average pledged access mana of the node.
func AveragePledgeAccessMap() mana.NodeMap {
	accessPledgeLock.RLock()
	defer accessPledgeLock.RUnlock()
	result := mana.NodeMap{}
	for k, pledges := range accessPledge {
		sum := 0.0
		for _, v := range pledges {
			sum += v
		}
		result[k] = sum / float64(len(pledges))
	}
	return result
}

// AveragePledgeConsensusMap returns the average pledged consensus mana of the node.
func AveragePledgeConsensusMap() mana.NodeMap {
	consensusPledgeLock.RLock()
	defer consensusPledgeLock.RUnlock()
	result := mana.NodeMap{}
	for k, pledges := range consensusPledge {
		sum := 0.0
		for _, v := range pledges {
			sum += v
		}
		result[k] = sum / float64(len(pledges))
	}
	return result
}

// addPledge populates the pledge logs.
func addPledge(event *mana.PledgedEvent) {
	amount := (event.AmountBM1 + event.AmountBM2) / 2
	switch event.Type {
	case mana.AccessMana:
		accessPledgeLock.Lock()
		defer accessPledgeLock.Unlock()
		pledges := accessPledge[event.NodeID]
		pledges = append(pledges, amount)
		accessPledge[event.NodeID] = pledges
	case mana.ConsensusMana:
		consensusPledgeLock.Lock()
		defer consensusPledgeLock.Unlock()
		pledges := consensusPledge[event.NodeID]
		pledges = append(pledges, amount)
		consensusPledge[event.NodeID] = pledges
	}
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

	accessMap, _ := manaPlugin.GetNeighborsMana(mana.AccessMana)
	accessSum, accessAvg := 0.0, 0.0
	for _, v := range accessMap {
		accessSum += v
	}
	if len(accessMap) > 0 {
		accessAvg = accessSum / float64(len(accessMap))
	}
	averageNeighborsAccess.Store(accessAvg)

	consensusMap, _ := manaPlugin.GetNeighborsMana(mana.ConsensusMana)
	consensusSum, consensusAvg := 0.0, 0.0
	for _, v := range consensusMap {
		consensusSum += v
	}
	if len(consensusMap) > 0 {
		consensusAvg = consensusSum / float64(len(consensusMap))
	}
	averageNeighborsConsensus.Store(consensusAvg)
}
