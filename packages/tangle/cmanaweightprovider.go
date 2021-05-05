package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
)

const (
	activeTimeThreshold  = 30 * time.Minute
	minimumManaThreshold = 0
)

type CManaWeightProvider struct {
	mutex             sync.RWMutex
	activeNodes       map[identity.ID]time.Time
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc) *CManaWeightProvider {
	return &CManaWeightProvider{
		activeNodes:       make(map[identity.ID]time.Time),
		manaRetrieverFunc: manaRetrieverFunc,
		timeRetrieverFunc: timeRetrieverFunc,
	}
}

func (c *CManaWeightProvider) Update(t time.Time, nodeID identity.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.activeNodes[nodeID].Before(t) {
		c.activeNodes[nodeID] = t
	}
}

func (c *CManaWeightProvider) Weight(message *Message) (weight, totalWeight float64) {
	weights, totalWeight := c.WeightsOfRelevantSupporters()
	return weights[identity.NewID(message.IssuerPublicKey())], totalWeight
}

func (c *CManaWeightProvider) WeightsOfRelevantSupporters() (weights map[identity.ID]float64, totalWeight float64) {
	weights = make(map[identity.ID]float64)

	mana := c.manaRetrieverFunc()
	targetTime := c.timeRetrieverFunc()
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, t := range c.activeNodes {
		if targetTime.Sub(t) > activeTimeThreshold {
			delete(c.activeNodes, nodeID)
			continue
		}

		nodeMana := mana[nodeID]
		if nodeMana <= minimumManaThreshold {
			continue
		}

		weights[nodeID] = nodeMana
		totalWeight += nodeMana
	}
	return weights, totalWeight
}

func (c *CManaWeightProvider) Shutdown() {
	//	TODO: possibly persist state
}

// ManaRetrieverFunc is a function type to retrieve consensus mana (e.g. via the mana plugin)
type ManaRetrieverFunc func() map[identity.ID]float64

type TimeRetrieverFunc func() time.Time
