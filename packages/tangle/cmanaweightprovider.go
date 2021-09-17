package tangle

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
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
	activeNodes       map[identity.ID]*activityLog
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activeNodes:       make(map[identity.ID]*activityLog),
		manaRetrieverFunc: manaRetrieverFunc,
		timeRetrieverFunc: timeRetrieverFunc,
	}
	//TODO:
	//if len(store) == 0 {
	//	return
	//}
	//
	//cManaWeightProvider.store = store[0]
	//
	//marshaledActiveNodes, err := cManaWeightProvider.store.Get(kvstore.Key(activeNodesKey))
	//if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
	//	panic(err)
	//}
	//// Load from storage if key was found.
	//if marshaledActiveNodes != nil {
	//	if cManaWeightProvider.activeNodes, err = activeNodesFromBytes(marshaledActiveNodes); err != nil {
	//		panic(err)
	//	}
	//	return
	//}

	return
}

// Update updates the underlying data structure and keeps track of active nodes.
func (c *CManaWeightProvider) Update(t time.Time, nodeID identity.ID) {
	fmt.Println("CManaWeightProvider.Update", t, nodeID)
	// We only want to log node activity that is relevant, i.e., node activity before TangleTime-activeTimeThreshold
	// does not matter anymore since the TangleTime advances towards the present/future.
	staleThreshold := c.timeRetrieverFunc().Add(-activeTimeThreshold)
	if t.Before(staleThreshold) {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	a, exists := c.activeNodes[nodeID]
	if !exists {
		a = newNodeActivity()
		c.activeNodes[nodeID] = a
	}

	a.Add(t)
	fmt.Println("CManaWeightProvider.Update done", t, nodeID)
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
	fmt.Println("WeightsOfRelevantSupporters", targetTime, mana)
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, al := range c.activeNodes {
		nodeMana := mana[nodeID]

		// Skip node if it does not fulfill minimumManaThreshold.
		if nodeMana <= minimumManaThreshold {
			continue
		}

		// Determine whether node was active in time window.
		if active, empty := al.Active(targetTime.Add(-activeTimeThreshold), targetTime); !active {
			fmt.Println("Not active", nodeID)
			if empty {
				delete(c.activeNodes, nodeID)
			}
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
		//TODO:
		//_ = c.store.Set(kvstore.Key(activeNodesKey), activeNodesToBytes(c.ActiveNodes()))
	}
}

// ActiveNodes returns the map of the active nodes.
func (c *CManaWeightProvider) ActiveNodes() (activeNodes map[identity.ID]time.Time) {
	activeNodes = make(map[identity.ID]time.Time)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	//TODO:
	//for nodeID, t := range c.activeNodes {
	//	activeNodes[nodeID] = t
	//}

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

// region activityLog //////////////////////////////////////////////////////////////////////////////////////////////////

// granularity defines the granularity in seconds with which we log node activities.
const granularity = 60

// timeToUnixGranularity converts a time t to a unix timestamp with granularity.
func timeToUnixGranularity(t time.Time) int64 {
	return t.Unix() / granularity
}

// activityLog is a time-based log of node activity. It stores information when a node was active and provides
// functionality to query for certain timeframes.
type activityLog struct {
	setTimes set.Set
	times    *minHeap
}

func newNodeActivity() *activityLog {
	var mh minHeap

	al := &activityLog{
		setTimes: set.New(),
		times:    &mh,
	}
	heap.Init(al.times)

	return al
}

// Add adds a node activity to the log.
func (a *activityLog) Add(t time.Time) (added bool) {
	u := timeToUnixGranularity(t)
	if !a.setTimes.Add(u) {
		return false
	}

	heap.Push(a.times, u)
	return true
}

// Active returns true if the node was active between lower and upper bound.
// It cleans up the log on the fly, meaning that old/stale times are deleted.
// If the log ends up empty after cleaning up, empty is set to true.
func (a *activityLog) Active(lowerBound, upperBound time.Time) (active, empty bool) {
	lb, ub := timeToUnixGranularity(lowerBound), timeToUnixGranularity(upperBound)

	for a.times.Len() > 0 {
		// Get the lowest element of the min-heap = the earliest time.
		activity := (*a.times)[0]

		// We clean up possible stale times < lowerBound because we don't need them anymore.
		if activity < lb {
			a.setTimes.Delete(activity)
			heap.Pop(a.times)
			continue
		}

		// Check if time is between lower and upper bound. Because of cleanup, activity >= lb is implicitly given.
		if activity <= ub {
			return true, false
		}
		// Otherwise, the node has active times in the future of upperBound.
		return false, false
	}

	// If the heap is empty, there's no activity anymore and the object might potentially be cleaned up.
	return false, true
}

// minHeap is a
type minHeap []int64

// Len is the number of elements in the collection.
func (h minHeap) Len() int {
	return len(h)
}

// Less reports whether the element with index i must sort before the element with index j.
func (h minHeap) Less(i, j int) bool {
	return h[i] < h[j]
}

// Swap swaps the elements with indexes i and j.
func (h minHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push pushes the element x onto the heap.
func (h *minHeap) Push(x interface{}) {
	*h = append(*h, x.(int64))
}

// Pop removes and returns the minimum element (according to Less) from the heap.
func (h *minHeap) Pop() interface{} {
	n := len(*h)
	x := (*h)[n-1]
	*h = (*h)[:n-1]
	return x
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
