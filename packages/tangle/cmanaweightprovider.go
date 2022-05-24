package tangle

import (
	"container/heap"
	"context"
	"fmt"
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
	activeTimeThreshold  = 30 * time.Minute
	minimumManaThreshold = 0
	activeNodesKey       = "WeightProviderActiveNodes"
)

// region CManaWeightProvider //////////////////////////////////////////////////////////////////////////////////////////

type NodesActivityLog map[identity.ID]*ActivityLog

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	store             kvstore.KVStore
	mutex             sync.RWMutex
	activeNodes       NodesActivityLog
	manaRetrieverFunc ManaRetrieverFunc
	timeRetrieverFunc TimeRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activeNodes:       make(NodesActivityLog),
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
		a = NewActivityLog()
		c.activeNodes[nodeID] = a
	}

	a.Add(t)
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
	targetTime := c.timeRetrieverFunc()
	lowerBoundTargetTime := targetTime.Add(-activeTimeThreshold)
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for nodeID, al := range c.activeNodes {
		nodeMana := mana[nodeID]

		// Determine whether node was active in time window.
		if active, empty := al.Active(lowerBoundTargetTime, targetTime); !active {
			if empty {
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

// granularity defines the granularity in seconds with which we log node activities.
const granularity = 60

// timeToUnixGranularity converts a time t to a unix timestamp with granularity.
func timeToUnixGranularity(t time.Time) int64 {
	return t.Unix() / granularity
}

// ActivityLog is a time-based log of node activity. It stores information when a node was active and provides
// functionality to query for certain timeframes.
type ActivityLog struct {
	setTimes set.Set[int64] `serix:"0,lengthPrefixType=uint32"`
	times    *minHeap
}

// NewActivityLog is the constructor for ActivityLog.
func NewActivityLog() *ActivityLog {
	var mh minHeap

	a := &ActivityLog{
		setTimes: set.New[int64](),
		times:    &mh,
	}
	heap.Init(a.times)

	return a
}

// Add adds a node activity to the log.
func (a *ActivityLog) Add(t time.Time) (added bool) {
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
func (a *ActivityLog) Active(lowerBound, upperBound time.Time) (active, empty bool) {
	lb, ub := timeToUnixGranularity(lowerBound), timeToUnixGranularity(upperBound)

	for a.times.Len() > 0 {
		// Get the lowest element of the min-heap = the earliest time.
		earliestActivity := (*a.times)[0]

		// We clean up possible stale times < lowerBound because we don't need them anymore.
		if earliestActivity < lb {
			a.setTimes.Delete(earliestActivity)
			heap.Pop(a.times)
			continue
		}

		// Check if time is between lower and upper bound. Because of cleanup, earliestActivity >= lb is implicitly given.
		if earliestActivity <= ub {
			return true, false
		}
		// Otherwise, the node has active times in the future of upperBound but is not currently active.
		return false, false
	}

	// If the heap is empty, there's no activity anymore and the object might potentially be cleaned up.
	return false, true
}

// Times returns all times stored in this ActivityLog.
func (a *ActivityLog) Times() (times []int64) {
	times = make([]int64, 0, a.times.Len())

	for _, u := range *a.times {
		times = append(times, u)
	}

	return times
}

// String returns a human-readable version of ActivityLog.
func (a *ActivityLog) String() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("ActivityLog(len=%d, elements=", a.times.Len()))
	for _, u := range *a.times {
		builder.WriteString(fmt.Sprintf("%d, ", u))
	}
	builder.WriteString(")")
	return builder.String()
}

// Clone clones the ActivityLog.
func (a *ActivityLog) Clone() *ActivityLog {
	clone := NewActivityLog()

	for _, u := range *a.times {
		clone.setTimes.Add(u)
		heap.Push(clone.times, u)
	}

	return clone
}

// Encode ActivityLog a serialized byte slice of the object.
func (a *ActivityLog) Encode() ([]byte, error) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a.setTimes, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes, nil
}

// Decode deserializes bytes into a valid object.
func (a *ActivityLog) Decode(data []byte) (bytesRead int, err error) {
	var mh minHeap

	a.setTimes = set.New[int64]()
	a.times = &mh
	bytesRead, err = serix.DefaultAPI.Decode(context.Background(), data, &a.setTimes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ActivityLog: %w", err)
		return
	}
	a.setTimes.ForEach(func(time int64) {
		heap.Push(a.times, time)
	})
	return
}

// minHeap is an int64 min heap.
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
