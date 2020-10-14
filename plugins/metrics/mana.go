package metrics

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

// PledgeLog is a log of base mana 1 and 2 pledges.
type PledgeLog struct {
	BM1Pledges []float64
	BM2Pledges []float64
}

// AddBM1 logs the value of base mana 1 pledged.
func (p *PledgeLog) AddBM1(val float64) {
	p.BM1Pledges = append(p.BM1Pledges, val)
}

// AddBM2 logs the value of base mana 2 pledged.
func (p *PledgeLog) AddBM2(val float64) {
	p.BM2Pledges = append(p.BM2Pledges, val)
}

// GetBM1Average returns the average base mana 1 pledged.
func (p *PledgeLog) GetBM1Average() float64 {
	if len(p.BM1Pledges) == 0 {
		return 0
	}
	var sum float64
	for _, val := range p.BM1Pledges {
		sum += val
	}
	return sum / float64(len(p.BM1Pledges))
}

// GetBM2Average returns the average base mana 2 pledged.
func (p *PledgeLog) GetBM2Average() float64 {
	if len(p.BM2Pledges) == 0 {
		return 0
	}
	var sum float64
	for _, val := range p.BM2Pledges {
		sum += val
	}
	return sum / float64(len(p.BM2Pledges))
}

// NodePledgeMap is a map of node and a list of mana pledges.
type NodePledgeMap map[identity.ID]*PledgeLog

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

// AveragePledgeConsensusBM returns the average pledged consensus base mana1 and base mann 2 of all nodes.
func AveragePledgeConsensusBM() (mana.NodeMap, mana.NodeMap) {
	consensusPledgeLock.RLock()
	defer consensusPledgeLock.RUnlock()
	bm1 := mana.NodeMap{}
	bm2 := mana.NodeMap{}

	for nodeID, pledgeLog := range consensusPledge {
		bm1[nodeID] = pledgeLog.GetBM1Average()
		bm2[nodeID] = pledgeLog.GetBM2Average()
	}
	return bm1, bm2
}

// AveragePledgeAccessBM returns the average pledged access base mana1 and base mann 2 of all nodes.
func AveragePledgeAccessBM() (mana.NodeMap, mana.NodeMap) {
	accessPledgeLock.RLock()
	defer accessPledgeLock.RUnlock()
	bm1 := mana.NodeMap{}
	bm2 := mana.NodeMap{}

	for nodeID, pledgeLog := range accessPledge {
		bm1[nodeID] = pledgeLog.GetBM1Average()
		bm2[nodeID] = pledgeLog.GetBM2Average()
	}
	return bm1, bm2
}

// addPledge populates the pledge logs for the node.
func addPledge(event *mana.PledgedEvent) {
	switch event.Type {
	case mana.AccessMana:
		accessPledgeLock.Lock()
		defer accessPledgeLock.Unlock()
		pledgeLog := accessPledge[event.NodeID]
		if pledgeLog == nil {
			pledgeLog = &PledgeLog{}
		}
		pledgeLog.AddBM1(event.AmountBM1)
		pledgeLog.AddBM2(event.AmountBM2)
		accessPledge[event.NodeID] = pledgeLog
	case mana.ConsensusMana:
		consensusPledgeLock.Lock()
		defer consensusPledgeLock.Unlock()
		pledgeLog := consensusPledge[event.NodeID]
		if pledgeLog == nil {
			pledgeLog = &PledgeLog{}
		}
		pledgeLog.AddBM1(event.AmountBM1)
		pledgeLog.AddBM2(event.AmountBM2)
		consensusPledge[event.NodeID] = pledgeLog
	}
}

func measureMana() {
	tmp := manaPlugin.GetAllManaMaps(mana.Mixed)
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

	accessMap, _ := manaPlugin.GetNeighborsMana(mana.AccessMana, mana.Mixed)
	accessSum, accessAvg := 0.0, 0.0
	for _, v := range accessMap {
		accessSum += v
	}
	if len(accessMap) > 0 {
		accessAvg = accessSum / float64(len(accessMap))
	}
	averageNeighborsAccess.Store(accessAvg)

	consensusMap, _ := manaPlugin.GetNeighborsMana(mana.ConsensusMana, mana.Mixed)
	consensusSum, consensusAvg := 0.0, 0.0
	for _, v := range consensusMap {
		consensusSum += v
	}
	if len(consensusMap) > 0 {
		consensusAvg = consensusSum / float64(len(consensusMap))
	}
	averageNeighborsConsensus.Store(consensusAvg)
}
