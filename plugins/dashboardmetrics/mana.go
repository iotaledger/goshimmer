package dashboardmetrics

import (
	"sync"

	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
)

// PledgeLog is a log of base mana 1 and 2 pledges.
type PledgeLog struct {
	AccessPledges    []int64
	ConsensusPledges []int64
}

// AddConsensus logs the value of consensus pledge (base mana 1) pledged.
func (p *PledgeLog) AddConsensus(val int64) {
	p.ConsensusPledges = append(p.ConsensusPledges, val)
}

// IssuerPledgeMap is a map of issuer and a list of mana pledges.
type IssuerPledgeMap map[identity.ID]*PledgeLog

var (
	// internal metrics for consensus mana
	consensusMap        manamodels.IssuerMap
	consensusPercentile atomic.Float64
	consensusLock       sync.RWMutex

	// internal metrics for pledges.
	pledges     = IssuerPledgeMap{}
	pledgesLock sync.RWMutex
)

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

func measureMana() {
	tmpConsensusMap := lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map())

	consensusLock.Lock()
	defer consensusLock.Unlock()
	consensusMap = tmpConsensusMap

	cPer := manamodels.Percentile(deps.Local.ID(), tmpConsensusMap)
	consensusPercentile.Store(cPer)

	neighbors := deps.P2Pmgr.AllNeighbors()
	var consensusSum, consensusCount int64

	for _, neighbor := range neighbors {
		neighborCMana := consensusMap[neighbor.ID()]
		if neighborCMana > 0 {
			consensusCount++
			consensusSum += neighborCMana
		}
	}
}
