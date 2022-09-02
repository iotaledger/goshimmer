package tangleold

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/types"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
)

const (
	minimumManaThreshold = 0
	activeNodesKey       = "WeightProviderActiveNodes"
	// activeEpochThreshold defines the activity window in number of epochs.
	activeEpochThreshold = 15
)

// region CManaWeightProvider //////////////////////////////////////////////////////////////////////////////////////////

// ActivityUpdatesCount stores the counters on how many times activity record was updated.
type ActivityUpdatesCount map[identity.ID]uint64

// CManaWeightProvider is a WeightProvider for consensus mana. It keeps track of active nodes based on their time-based
// activity in relation to activeTimeThreshold.
type CManaWeightProvider struct {
	store                       kvstore.KVStore
	mutex                       sync.RWMutex
	activityLog                 *epoch.NodesActivityLog
	updatedActivityCount        *shrinkingmap.ShrinkingMap[epoch.Index, ActivityUpdatesCount]
	manaRetrieverFunc           ManaRetrieverFunc
	timeRetrieverFunc           TimeRetrieverFunc
	confirmedEpochRetrieverFunc ConfirmedEpochRetrieverFunc
}

// NewCManaWeightProvider is the constructor for CManaWeightProvider.
func NewCManaWeightProvider(manaRetrieverFunc ManaRetrieverFunc, timeRetrieverFunc TimeRetrieverFunc, confirmedEpochRetrieverFunc ConfirmedEpochRetrieverFunc, store ...kvstore.KVStore) (cManaWeightProvider *CManaWeightProvider) {
	cManaWeightProvider = &CManaWeightProvider{
		activityLog:                 epoch.NewNodesActivityLog(),
		updatedActivityCount:        shrinkingmap.New[epoch.Index, ActivityUpdatesCount](shrinkingmap.WithShrinkingThresholdCount(100)),
		manaRetrieverFunc:           manaRetrieverFunc,
		timeRetrieverFunc:           timeRetrieverFunc,
		confirmedEpochRetrieverFunc: confirmedEpochRetrieverFunc,
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

		if err = cManaWeightProvider.activityLog.FromBytes(marshaledActiveNodes); err != nil {
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

	a, exists := c.activityLog.Get(ei)
	if !exists {
		a = epoch.NewActivityLog()
		c.activityLog.Set(ei, a)
	}

	a.Add(nodeID)

	c.updateActivityCount(ei, nodeID, 1)
}

// Remove updates the underlying data structure by decreasing updatedActivityCount and removing node from active list if no activity left.
func (c *CManaWeightProvider) Remove(ei epoch.Index, nodeID identity.ID, updatedActivityCount uint64) (removed bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	epochUpdatesCount, exist := c.updatedActivityCount.Get(ei)
	if exist {
		_, exists := epochUpdatesCount[nodeID]
		if exists {
			epochUpdatesCount[nodeID] -= updatedActivityCount
		}
	}
	// if that was the last activity for this node in the ei epoch, then remove it from activity list
	if epochUpdatesCount[nodeID] == 0 {
		if a, exists := c.activityLog.Get(ei); exists {
			a.Remove(nodeID)
			return true
		}
	}
	return false
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

	lowerBoundEpoch, upperBoundEpoch := c.activityBoundaries()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// nodes mana is counted only once for total weight calculation
	totalWeightOnce := make(map[identity.ID]types.Empty)
	for ei := lowerBoundEpoch; ei <= upperBoundEpoch; ei++ {
		al, exists := c.activityLog.Get(ei)
		if !exists {
			continue
		}
		al.ForEach(func(nodeID identity.ID) error {
			nodeMana := mana[nodeID]
			// Do this check after determining whether a node was active because otherwise we would never clean up
			// the ActivityLog of nodes lower than the threshold.
			// Skip node if it does not fulfill minimumManaThreshold.
			if nodeMana <= minimumManaThreshold {
				return nil
			}

			weights[nodeID] = nodeMana
			if _, notFirstTime := totalWeightOnce[nodeID]; !notFirstTime {
				totalWeight += nodeMana
				totalWeightOnce[nodeID] = types.Void
			}
			return nil
		})
	}
	pruningPoint := c.confirmedEpochRetrieverFunc() - epoch.Index(activeEpochThreshold)
	c.clean(pruningPoint)

	return weights, totalWeight
}

// SnapshotEpochActivity returns the activity log for snapshotting.
func (c *CManaWeightProvider) SnapshotEpochActivity(epochDiffIndex epoch.Index) (epochActivity epoch.SnapshotEpochActivity) {
	epochActivity = epoch.NewSnapshotEpochActivity()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.activityLog.ForEach(func(ei epoch.Index, activity *epoch.ActivityLog) bool {
		activity.ForEach(func(nodeID identity.ID) error {
			// we save only activity log up to epochDiffIndex as it is the last snapshotted epoch
			if ei > epochDiffIndex {
				return nil
			}
			if _, ok := epochActivity[ei]; !ok {
				epochActivity[ei] = epoch.NewSnapshotNodeActivity()
			}
			// Snapshot activity counts
			activityCount, exists := c.updatedActivityCount.Get(ei)
			if exists {
				epochActivity[ei].SetNodeActivity(nodeID, activityCount[nodeID])
			}
			return nil
		})
		return true
	})

	return
}

// Shutdown shuts down the WeightProvider and persists its state.
func (c *CManaWeightProvider) Shutdown() {
	if c.store != nil {
		activeNodes := c.activeNodes()
		_ = c.store.Set(kvstore.Key(activeNodesKey), activeNodes.Bytes())
	}
}

// LoadActiveNodes loads the activity log to weight provider.
func (c *CManaWeightProvider) LoadActiveNodes(loadedActiveNodes epoch.SnapshotEpochActivity) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for ei, epochActivity := range loadedActiveNodes {
		var activityLog *epoch.ActivityLog
		var ok bool
		if activityLog, ok = c.activityLog.Get(ei); !ok {
			activityLog = epoch.NewActivityLog()
			c.activityLog.Set(ei, activityLog)
		}
		for nodeID, activityCount := range epochActivity.NodesLog() {
			activityLog.Add(nodeID)
			c.updateActivityCount(ei, nodeID, activityCount)
		}
	}
}

// ManaRetrieverFunc is a function type to retrieve consensus mana (e.g. via the mana plugin).
type ManaRetrieverFunc func() map[identity.ID]float64

// TimeRetrieverFunc is a function type to retrieve the time.
type TimeRetrieverFunc func() time.Time

// ConfirmedEpochRetrieverFunc is a function type to retrieve the confirmed epoch index.
type ConfirmedEpochRetrieverFunc func() epoch.Index

// activeNodes returns the map of the active nodes.
func (c *CManaWeightProvider) activeNodes() (activeNodes *epoch.NodesActivityLog) {
	activeNodes = epoch.NewNodesActivityLog()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.activityLog.ForEach(func(ei epoch.Index, activity *epoch.ActivityLog) bool {
		activeNodes.Set(ei, activity.Clone())
		return true
	})

	return activeNodes
}

func (c *CManaWeightProvider) activityBoundaries() (lowerBoundEpoch, upperBoundEpoch epoch.Index) {
	currentTime := c.timeRetrieverFunc()
	upperBoundEpoch = epoch.IndexFromTime(currentTime)
	lowerBoundEpoch = upperBoundEpoch - activeEpochThreshold
	if lowerBoundEpoch < 0 {
		lowerBoundEpoch = 0
	}
	return
}

// clean removes all activity logs for epochs lower than provided bound.
func (c *CManaWeightProvider) clean(cutoffEpoch epoch.Index) {
	c.activityLog.ForEachKey(func(ei epoch.Index) bool {
		if ei < cutoffEpoch {
			c.activityLog.Delete(ei)
		}
		return true
	})
	// clean also the updates counting map
	c.updatedActivityCount.ForEach(func(ei epoch.Index, count ActivityUpdatesCount) bool {
		if ei < cutoffEpoch {
			c.updatedActivityCount.Delete(ei)
		}
		return true
	})
}

func (c *CManaWeightProvider) updateActivityCount(ei epoch.Index, nodeID identity.ID, increase uint64) {
	_, exist := c.updatedActivityCount.Get(ei)
	if !exist {
		c.updatedActivityCount.Set(ei, make(ActivityUpdatesCount))
	}
	epochUpdatesCount, _ := c.updatedActivityCount.Get(ei)
	epochUpdatesCount[nodeID] += increase
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
