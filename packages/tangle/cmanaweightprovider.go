package tangle

import (
	"context"
	"fmt"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serix"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(NodesActivityLog{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))

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

type NodesActivityLog map[identity.ID]*ActivityLog

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	store               kvstore.KVStore
	mutex               sync.RWMutex
	activeNodes         NodesActivityLog
	manaRetrieverFunc   ManaRetrieverFunc
	epochRetrieverFunc  EpochRetrieverFunc
	manaEpochDelayParam uint
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, epochRetrieverFunc EpochRetrieverFunc, manaEpochDelay uint, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activeNodes:         make(NodesActivityLog),
		manaRetrieverFunc:   manaRetrieverFunc,
		epochRetrieverFunc:  epochRetrieverFunc,
		manaEpochDelayParam: manaEpochDelay,
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
		a = NewActivityLog()
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

// Weight returns the weight and total weight for the given message.
func (c *CManaWeightProvider) Weight(message *Message) (weight, totalWeight float64) {
	weights, totalWeight := c.WeightsOfRelevantVoters()
	return weights[identity.NewID(message.IssuerPublicKey())], totalWeight
}

// WeightsOfRelevantVoters returns all relevant weights.
func (c *CManaWeightProvider) WeightsOfRelevantVoters() (weights map[identity.ID]float64, totalWeight float64) {
	weights = make(map[identity.ID]float64)

	mana := c.manaRetrieverFunc()

	latestCommutableEI := c.epochRetrieverFunc()
	lowerBoundEpoch := latestCommutableEI - activeEpochThreshold - epoch.Index(c.manaEpochDelayParam)
	upperBoundEpoch := latestCommutableEI - epoch.Index(c.manaEpochDelayParam)

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
func (c *CManaWeightProvider) ActiveNodes() (activeNodes NodesActivityLog) {
	activeNodes = make(NodesActivityLog)

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

//
type EpochRetrieverFunc func() epoch.Index

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region activeNodes //////////////////////////////////////////////////////////////////////////////////////////////////

func activeNodesFromBytes(data []byte) (activeNodes NodesActivityLog, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &activeNodes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse activeNodes: %w", err)
		return
	}
	return
}

func activeNodesToBytes(activeNodes NodesActivityLog) []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), activeNodes, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ActivityLog //////////////////////////////////////////////////////////////////////////////////////////////////

// ActivityLog is a time-based log of node activity. It stores information when a node was active and provides
// functionality to query for certain timeframes.
type ActivityLog struct {
	setEpochs set.Set[epoch.Index] `serix:"0,lengthPrefixType=uint32"`
}

// NewActivityLog is the constructor for ActivityLog.
func NewActivityLog() *ActivityLog {

	a := &ActivityLog{
		setEpochs: set.New[epoch.Index](),
	}

	return a
}

// Add adds a node activity to the log.
func (a *ActivityLog) Add(ei epoch.Index) (added bool) {
	if !a.setEpochs.Add(ei) {
		return false
	}

	return true
}

// Remove removes a node activity from the log.
func (a *ActivityLog) Remove(ei epoch.Index) (removed bool) {
	if !a.setEpochs.Delete(ei) {
		return false
	}

	return true
}

// Active returns true if the node was active between lower and upper bound.
// It cleans up the log on the fly, meaning that old/stale times are deleted.
// If the log ends up empty after cleaning up, empty is set to true.
func (a *ActivityLog) Active(lowerBound, upperBound epoch.Index) (active bool) {
	for ei := lowerBound; ei <= upperBound; ei++ {
		if a.setEpochs.Has(ei) {
			active = true
		}
	}

	return
}

func (a *ActivityLog) Clean(cutoff epoch.Index) (empty bool) {
	// we remove all activity records below lowerBound as we will no longer need it
	a.setEpochs.ForEach(func(ei epoch.Index) {
		if ei < cutoff {
			a.setEpochs.Delete(ei)
		}
	})
	if a.setEpochs.Size() == 0 {
		return true
	}
	return
}

// Epochs returns all epochs stored in this ActivityLog.
func (a *ActivityLog) Epochs() (epochs []epoch.Index) {
	epochs = make([]epoch.Index, 0, a.setEpochs.Size())

	// insert in order
	a.setEpochs.ForEach(func(ei epoch.Index) {
		idx := sort.Search(len(epochs), func(i int) bool { return epochs[i] >= ei })
		epochs = append(epochs[:idx+1], epochs[idx:]...)
		epochs[idx] = ei
	})

	return
}

// String returns a human-readable version of ActivityLog.
func (a *ActivityLog) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("ActivityLog(len=%d, elements=", a.setEpochs.Size()))
	ordered := a.Epochs()
	for _, u := range ordered {
		builder.WriteString(fmt.Sprintf("%d, ", u))
	}
	builder.WriteString(")")
	return builder.String()
}

// Clone clones the ActivityLog.
func (a *ActivityLog) Clone() *ActivityLog {
	clone := NewActivityLog()

	a.setEpochs.ForEach(func(ei epoch.Index) {
		clone.setEpochs.Add(ei)
	})

	return clone
}

// Encode ActivityLog a serialized byte slice of the object.
func (a *ActivityLog) Encode() ([]byte, error) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a.setEpochs, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes, nil
}

// Decode deserializes bytes into a valid object.
func (a *ActivityLog) Decode(data []byte) (bytesRead int, err error) {

	a.setEpochs = set.New[epoch.Index]()
	bytesRead, err = serix.DefaultAPI.Decode(context.Background(), data, &a.setEpochs, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ActivityLog: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
