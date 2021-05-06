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

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	mutex             sync.RWMutex
	activeNodes       map[identity.ID]time.Time
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc) *CManaWeightProvider {
	return &CManaWeightProvider{
		activeNodes:       make(map[identity.ID]time.Time),
		manaRetrieverFunc: manaRetrieverFunc,
		timeRetrieverFunc: timeRetrieverFunc,
	}
}

// Update updates the underlying data structure and keeps track of active nodes.
func (c *CManaWeightProvider) Update(t time.Time, nodeID identity.ID) {
	if c.timeRetrieverFunc().Before(t) {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.activeNodes[nodeID].Before(t) {
		c.activeNodes[nodeID] = t
	}
}

// Weight returns the weight and total weight for the given message.
func (c *CManaWeightProvider) Weight(message *Message) (weight, totalWeight float64) {
	weights, totalWeight := c.WeightsOfRelevantSupporters()
	return weights[identity.NewID(message.IssuerPublicKey())], totalWeight
}

// WeightsOfRelevantSupporters returns all relevant weights.
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

// Shutdown shuts down the WeightProvider and persists its state.
func (c *CManaWeightProvider) Shutdown() {
	//	TODO: possibly persist state
}

// ActiveNodes returns the map of the active nodes.
func (c *CManaWeightProvider) ActiveNodes() (activeNodes map[identity.ID]time.Time) {
	activeNodes = make(map[identity.ID]time.Time)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, t := range c.activeNodes {
		activeNodes[nodeID] = t
	}

	return activeNodes
}

// ManaRetrieverFunc is a function type to retrieve consensus mana (e.g. via the mana plugin)
type ManaRetrieverFunc func() map[identity.ID]float64

// TimeRetrieverFunc is a function type to retrieve the time.
type TimeRetrieverFunc func() time.Time
