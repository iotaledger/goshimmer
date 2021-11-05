package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/manarefresher"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// PledgeLog is a log of base mana 1 and 2 pledges.
type PledgeLog struct {
	AccessPledges    []float64
	ConsensusPledges []float64
}

// AddAccess logs the value of access pledge (base mana 2) pledged.
func (p *PledgeLog) AddAccess(val float64) {
	p.AccessPledges = append(p.AccessPledges, val)
}

// AddConsensus logs the value of consensus pledge (base mana 1) pledged.
func (p *PledgeLog) AddConsensus(val float64) {
	p.ConsensusPledges = append(p.ConsensusPledges, val)
}

// GetAccessAverage returns the average access mana pledge of a node.
func (p *PledgeLog) GetAccessAverage() float64 {
	if len(p.AccessPledges) == 0 {
		return 0
	}
	var sum float64
	for _, val := range p.AccessPledges {
		sum += val
	}
	return sum / float64(len(p.AccessPledges))
}

// GetConsensusAverage returns the consensus mana pledged.
func (p *PledgeLog) GetConsensusAverage() float64 {
	if len(p.ConsensusPledges) == 0 {
		return 0
	}
	var sum float64
	for _, val := range p.ConsensusPledges {
		sum += val
	}
	return sum / float64(len(p.ConsensusPledges))
}

// NodePledgeMap is a map of node and a list of mana pledges.
type NodePledgeMap map[identity.ID]*PledgeLog

var (
	// internal metrics for access mana
	accessMap        mana.NodeMap
	accessPercentile atomic.Float64
	accessLock       sync.RWMutex

	// internal metrics for consensus mana
	consensusMap        mana.NodeMap
	consensusPercentile atomic.Float64
	consensusLock       sync.RWMutex

	// internal metrics for neighbor's mana
	averageNeighborsAccess    atomic.Float64
	averageNeighborsConsensus atomic.Float64

	// internal metrics for delegated mana.
	delegationAmount atomic.Uint64

	// internal metrics for pledges.
	pledges     = NodePledgeMap{}
	pledgesLock sync.RWMutex
)

// AccessPercentile returns the top percentile the node belongs to in terms of access mana holders.
func AccessPercentile() float64 {
	return accessPercentile.Load()
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
	return consensusMap[deps.Local.ID()]
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

// AveragePledgeConsensus returns the average pledged consensus base mana of all nodes.
func AveragePledgeConsensus() mana.NodeMap {
	pledgesLock.RLock()
	defer pledgesLock.RUnlock()
	result := mana.NodeMap{}
	for nodeID, pledgeLog := range pledges {
		result[nodeID] = pledgeLog.GetConsensusAverage()
	}
	return result
}

// AveragePledgeAccess returns the average pledged access base mana of all nodes.
func AveragePledgeAccess() mana.NodeMap {
	pledgesLock.RLock()
	defer pledgesLock.RUnlock()
	result := mana.NodeMap{}
	for nodeID, pledgeLog := range pledges {
		result[nodeID] = pledgeLog.GetAccessAverage()
	}
	return result
}

// DelegatedMana returns how much mana is currently delegated to the node.
func DelegatedMana() uint64 {
	return delegationAmount.Load()
}

// addPledge populates the pledge logs for the node.
func addPledge(event *mana.PledgedEvent) {
	pledgesLock.Lock()
	defer pledgesLock.Unlock()
	pledgeLog := pledges[event.NodeID]
	if pledgeLog == nil {
		pledgeLog = &PledgeLog{}
	}
	switch event.ManaType {
	case mana.AccessMana:
		pledgeLog.AddAccess(event.Amount)
	case mana.ConsensusMana:
		pledgeLog.AddConsensus(event.Amount)
	}
	pledges[event.NodeID] = pledgeLog
}

func measureMana() {
	tmp, _ := manaPlugin.GetAllManaMaps()
	accessLock.Lock()
	defer accessLock.Unlock()
	accessMap = tmp[mana.AccessMana]
	aPer, _ := accessMap.GetPercentile(deps.Local.ID())
	accessPercentile.Store(aPer)
	consensusLock.Lock()
	defer consensusLock.Unlock()
	consensusMap = tmp[mana.ConsensusMana]
	cPer, _ := consensusMap.GetPercentile(deps.Local.ID())
	consensusPercentile.Store(cPer)
	neighbors := deps.GossipMgr.AllNeighbors()
	neighborAccessMap, _ := manaPlugin.GetNeighborsMana(mana.AccessMana, neighbors)
	accessSum, accessAvg := 0.0, 0.0
	for _, v := range neighborAccessMap {
		accessSum += v
	}
	if len(neighborAccessMap) > 0 {
		accessAvg = accessSum / float64(len(neighborAccessMap))
	}
	averageNeighborsAccess.Store(accessAvg)

	neighborConsensusMap, _ := manaPlugin.GetNeighborsMana(mana.ConsensusMana, neighbors)
	consensusSum, consensusAvg := 0.0, 0.0
	for _, v := range neighborConsensusMap {
		consensusSum += v
	}
	if len(neighborConsensusMap) > 0 {
		consensusAvg = consensusSum / float64(len(neighborConsensusMap))
	}
	averageNeighborsConsensus.Store(consensusAvg)

	delegationAmount.Store(manarefresher.TotalDelegatedFunds())
}
