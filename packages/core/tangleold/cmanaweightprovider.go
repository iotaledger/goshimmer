package tangleold

import (
	"context"
	"fmt"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serix"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(epoch.NodesActivityLog{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))
	if err != nil {
		panic(fmt.Errorf("error registering GenericDataPayload type settings: %w", err))
	}
}

const (
	minimumManaThreshold = 0
	activeNodesKey       = "WeightProviderActiveNodes"
	activeEpochThreshold = 2
)

// region CManaWeightProvider //////////////////////////////////////////////////////////////////////////////////////////

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	store             kvstore.KVStore
	mutex             sync.RWMutex
	activeNodes       epoch.NodesActivityLog
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activeNodes:       make(epoch.NodesActivityLog),
		manaRetrieverFunc: manaRetrieverFunc,
		timeRetrieverFunc: timeRetrieverFunc,
	}

	if len(store) == 0 {
		return
	}

	cManaWeightProvider.store = store[0]

	marshaledActiveNodes, err := cManaWeightProvider.store.Get(kvstore.Key(activeNodesKey))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}
	// Load from storage if key was found.
	if marshaledActiveNodes != nil {
		if cManaWeightProvider.activeNodes, err = activeNodesFromBytes(marshaledActiveNodes); err != nil {
			panic(err)
		}
		return
	}

	return
}

// Update updates the underlying data structure and keeps track of active nodes.
func (c *CManaWeightProvider) Update(ei epoch.Index, nodeID identity.ID) {
	// We don't check if the epoch index is too old, as this is handled by the NotarizationManager
	c.mutex.Lock()
	defer c.mutex.Unlock()

	a, exists := c.activeNodes[nodeID]
	if !exists {
		a = epoch.NewActivityLog()
		c.activeNodes[nodeID] = a
	}

	a.Add(ei)
}

// Remove updates the underlying data structure by removing node from activity list.
func (c *CManaWeightProvider) Remove(ei epoch.Index, nodeID identity.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// we allow for removing nodes from activity list due to orphanage event
	if a, exists := c.activeNodes[nodeID]; exists {
		a.Remove(ei)
	}
}

// Weight returns the weight and total weight for the given block.
func (c *CManaWeightProvider) Weight(block *Block) (weight, totalWeight float64) {
	weights, totalWeight := c.WeightsOfRelevantVoters()
	return weights[identity.NewID(block.IssuerPublicKey())], totalWeight
}

// WeightsOfRelevantVoters returns all relevant weights.
func (c *CManaWeightProvider) WeightsOfRelevantVoters() (weights map[identity.ID]float64, totalWeight float64) {
	weights = make(map[identity.ID]float64)

	mana := c.manaRetrieverFunc()

	currentTime := c.timeRetrieverFunc()
	upperBoundEpoch := epoch.IndexFromTime(currentTime)
	lowerBoundEpoch := upperBoundEpoch - activeEpochThreshold
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, al := range c.activeNodes {
		nodeMana := mana[nodeID]

		// Determine whether node was active in time window.
		if active := al.Active(lowerBoundEpoch, upperBoundEpoch); !active {
			if empty := al.Clean(lowerBoundEpoch); empty {
				delete(c.activeNodes, nodeID)
			}
			continue
		}
		// Do this check after determining whether a node was active because otherwise we would never clean up
		// the ActivityLog of nodes lower than the threshold.
		// Skip node if it does not fulfill minimumManaThreshold.
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
	if c.store != nil {
		_ = c.store.Set(kvstore.Key(activeNodesKey), activeNodesToBytes(c.ActiveNodes()))
	}
}

// ActiveNodes returns the map of the active nodes.
func (c *CManaWeightProvider) ActiveNodes() (activeNodes epoch.NodesActivityLog) {
	activeNodes = make(epoch.NodesActivityLog)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, al := range c.activeNodes {
		activeNodes[nodeID] = al.Clone()
	}

	return activeNodes
}

// ManaRetrieverFunc is a function type to retrieve consensus mana (e.g. via the mana plugin).
type ManaRetrieverFunc func() map[identity.ID]float64

// TimeRetrieverFunc is a function type to retrieve the time.
type TimeRetrieverFunc func() time.Time

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region activeNodes //////////////////////////////////////////////////////////////////////////////////////////////////

func activeNodesFromBytes(data []byte) (activeNodes epoch.NodesActivityLog, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &activeNodes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse activeNodes: %w", err)
		return
	}
	return
}

func activeNodesToBytes(activeNodes epoch.NodesActivityLog) []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), activeNodes, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
