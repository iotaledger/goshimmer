package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
)

// PledgeLog is a log of base mana 1 and 2 pledges.
type PledgeLog struct {
	AccessPledges    []int64
	ConsensusPledges []int64
}

// AddAccess logs the value of access pledge (base mana 2) pledged.
func (p *PledgeLog) AddAccess(val int64) {
	p.AccessPledges = append(p.AccessPledges, val)
}

// AddConsensus logs the value of consensus pledge (base mana 1) pledged.
func (p *PledgeLog) AddConsensus(val int64) {
	p.ConsensusPledges = append(p.ConsensusPledges, val)
}

// GetAccessAverage returns the average access mana pledge of a issuer.
func (p *PledgeLog) GetAccessAverage() int64 {
	if len(p.AccessPledges) == 0 {
		return 0
	}
	var sum int64
	for _, val := range p.AccessPledges {
		sum += val
	}
	return sum / int64(len(p.AccessPledges))
}

// GetConsensusAverage returns the consensus mana pledged.
func (p *PledgeLog) GetConsensusAverage() int64 {
	if len(p.ConsensusPledges) == 0 {
		return 0
	}
	var sum int64
	for _, val := range p.ConsensusPledges {
		sum += val
	}
	return sum / int64(len(p.ConsensusPledges))
}

// IssuerPledgeMap is a map of issuer and a list of mana pledges.
type IssuerPledgeMap map[identity.ID]*PledgeLog

var (
	// internal metrics for access mana
	accessMap        manamodels.IssuerMap
	accessPercentile atomic.Float64
	accessLock       sync.RWMutex

	// internal metrics for consensus mana
	consensusMap        manamodels.IssuerMap
	consensusPercentile atomic.Float64
	consensusLock       sync.RWMutex

	// internal metrics for neighbor's mana
	averageNeighborsAccess    atomic.Int64
	averageNeighborsConsensus atomic.Int64

	// internal metrics for pledges.
	pledges     = IssuerPledgeMap{}
	pledgesLock sync.RWMutex
)

// AccessPercentile returns the top percentile the issuer belongs to in terms of access mana holders.
func AccessPercentile() float64 {
	return accessPercentile.Load()
}

// AccessManaMap returns the access mana of the whole network.
func AccessManaMap() manamodels.IssuerMap {
	accessLock.RLock()
	defer accessLock.RUnlock()
	result := manamodels.IssuerMap{}
	for k, v := range accessMap {
		result[k] = v
	}
	return result
}

// ConsensusPercentile returns the top percentile the issuer belongs to in terms of consensus mana holders.
func ConsensusPercentile() float64 {
	return consensusPercentile.Load()
}

// OwnConsensusMana returns the consensus mana of the issuer.
func OwnConsensusMana() int64 {
	consensusLock.RLock()
	defer consensusLock.RUnlock()
	return consensusMap[deps.Local.ID()]
}

// ConsensusManaMap returns the consensus mana of the whole network.
func ConsensusManaMap() manamodels.IssuerMap {
	consensusLock.RLock()
	defer consensusLock.RUnlock()
	result := manamodels.IssuerMap{}
	for k, v := range consensusMap {
		result[k] = v
	}
	return result
}

// AverageNeighborsAccess returns the average access mana of the issuers neighbors.
func AverageNeighborsAccess() int64 {
	return averageNeighborsAccess.Load()
}

// AverageNeighborsConsensus returns the average consensus mana of the issuers neighbors.
func AverageNeighborsConsensus() int64 {
	return averageNeighborsConsensus.Load()
}

// AveragePledgeConsensus returns the average pledged consensus base mana of all issuers.
func AveragePledgeConsensus() manamodels.IssuerMap {
	pledgesLock.RLock()
	defer pledgesLock.RUnlock()
	result := manamodels.IssuerMap{}
	for issuerID, pledgeLog := range pledges {
		result[issuerID] = pledgeLog.GetConsensusAverage()
	}
	return result
}

// AveragePledgeAccess returns the average pledged access base mana of all issuers.
func AveragePledgeAccess() manamodels.IssuerMap {
	pledgesLock.RLock()
	defer pledgesLock.RUnlock()
	result := manamodels.IssuerMap{}
	for issuerID, pledgeLog := range pledges {
		result[issuerID] = pledgeLog.GetAccessAverage()
	}
	return result
}

// addPledge populates the pledge logs for the issuer.
func addPledge(event *mana.PledgedEvent) {
	pledgesLock.Lock()
	defer pledgesLock.Unlock()
	pledgeLog := pledges[event.IssuerID]
	if pledgeLog == nil {
		pledgeLog = &PledgeLog{}
	}
	switch event.ManaType {
	case manamodels.AccessMana:
		pledgeLog.AddAccess(event.Amount)
	case manamodels.ConsensusMana:
		pledgeLog.AddConsensus(event.Amount)
	}
	pledges[event.IssuerID] = pledgeLog
}

func measureMana() {
	tmp, _ := deps.Protocol.Instance().Engine.CongestionControl.GetAllManaMaps()
	accessLock.Lock()
	defer accessLock.Unlock()
	accessMap = tmp[manamodels.AccessMana]
	aPer, _ := accessMap.GetPercentile(deps.Local.ID())
	accessPercentile.Store(aPer)
	consensusLock.Lock()
	defer consensusLock.Unlock()
	consensusMap = tmp[manamodels.ConsensusMana]
	cPer, _ := consensusMap.GetPercentile(deps.Local.ID())
	consensusPercentile.Store(cPer)

	neighbors := deps.P2Pmgr.AllNeighbors()
	var accessSum, accessAvg, accessCount int64
	var consensusSum, consensusAvg, consensusCount int64

	for _, neighbor := range neighbors {
		neighborAMana, _, _ := deps.Protocol.Instance().Engine.CongestionControl.GetAccessMana(neighbor.ID())
		if neighborAMana > 0 {
			accessCount++
			accessSum += neighborAMana
		}

		neighborCMana, _, _ := deps.Protocol.Instance().Engine.CongestionControl.GetConsensusMana(neighbor.ID())
		if neighborCMana > 0 {
			consensusCount++
			consensusSum += neighborCMana
		}
	}
	if accessCount > 0 {
		accessAvg = accessSum / accessCount
	}
	if consensusCount > 0 {
		consensusAvg = consensusSum / consensusCount
	}
	averageNeighborsAccess.Store(accessAvg)
	averageNeighborsConsensus.Store(consensusAvg)
}
