package tangle

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
)

const (
	activeTimeThreshold  = 30 * time.Minute
	minimumManaThreshold = 0
	activeNodesKey       = "WeightProviderActiveNodes"
)

// region CManaWeightProvider //////////////////////////////////////////////////////////////////////////////////////////

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	store             kvstore.KVStore
	mutex             sync.RWMutex
	activeNodes       map[identity.ID]time.Time
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activeNodes:       make(map[identity.ID]time.Time),
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
func (c *CManaWeightProvider) Update(t time.Time, nodeID identity.ID) {
	// We don't want to add nodes as active if their message is in the future of the TangleTime so that we get a sliding
	// window of how the tangle emerged.
	if t.After(c.timeRetrieverFunc()) {
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
	if c.store != nil {
		_ = c.store.Set(kvstore.Key(activeNodesKey), activeNodesToBytes(c.ActiveNodes()))
	}
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region activeNodes //////////////////////////////////////////////////////////////////////////////////////////////////

func activeNodesFromBytes(bytes []byte) (activeNodes map[identity.ID]time.Time, err error) {
	activeNodes = make(map[identity.ID]time.Time)

	marshalUtil := marshalutil.New(bytes)
	count, err := marshalUtil.ReadUint32()
	if err != nil {
		err = errors.Errorf("failed to parse weight (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	for i := uint32(0); i < count; i++ {
		nodeID, idErr := identity.IDFromMarshalUtil(marshalUtil)
		if idErr != nil {
			err = errors.Errorf("failed to parse ID from MarshalUtil: %w", idErr)
			return
		}

		lastSeenTime, timeErr := marshalUtil.ReadTime()
		if timeErr != nil {
			err = errors.Errorf("failed to parse weight (%v): %w", timeErr, cerrors.ErrParseBytesFailed)
			return
		}

		activeNodes[nodeID] = lastSeenTime
	}

	return
}

func activeNodesToBytes(activeNodes map[identity.ID]time.Time) []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint32(uint32(len(activeNodes)))
	for nodeID, t := range activeNodes {
		marshalUtil.Write(nodeID)
		marshalUtil.WriteTime(t)
	}

	return marshalUtil.Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
