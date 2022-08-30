package tangleold

import (
	"math/rand"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

func TestActiveNodesMarshalling(t *testing.T) {
	nodes := map[string]identity.ID{
		"node1": identity.GenerateIdentity().ID(),
		"node2": identity.GenerateIdentity().ID(),
		"node3": identity.GenerateIdentity().ID(),
	}

	activeNodes := epoch.NewNodesActivityLog()

	for i := 0; i < 100; i++ {
		ei := epoch.Index(i)
		al := epoch.NewActivityLog()
		activeNodes.Set(ei, al)
		weight := 1.0
		for _, nodeID := range nodes {
			if rand.Float64() < 0.1*weight {
				al.Add(nodeID)
			}
			weight += 1
		}
	}

	activeNodesBytes := activeNodes.Bytes()
	require.NotNil(t, activeNodesBytes)
	activeNodes2 := epoch.NewNodesActivityLog()
	err := activeNodes2.FromBytes(activeNodesBytes)
	require.NoError(t, err)
	activeNodes.ForEach(func(ei epoch.Index, activity *epoch.ActivityLog) bool {
		activity2, exists := activeNodes2.Get(ei)
		require.True(t, exists)
		assert.EqualValues(t, activity.Size(), activity2.Size())
		return true
	})
}

func TestCManaWeightProvider(t *testing.T) {
	nodes := map[string]identity.ID{
		"1": identity.GenerateIdentity().ID(),
		"2": identity.GenerateIdentity().ID(),
		"3": identity.GenerateIdentity().ID(),
	}

	manaRetrieverFunc := func() map[identity.ID]float64 {
		return map[identity.ID]float64{
			nodes["1"]: 20,
			nodes["2"]: 50,
			nodes["3"]: 30,
		}
	}

	startEpoch := epoch.IndexFromTime(time.Now())
	epochManager := &struct {
		ei epoch.Index
	}{
		ei: startEpoch,
	}
	epochRetrieverFunc := func() epoch.Index { return epochManager.ei }
	timeRetrieverFunc := func() time.Time { return epochRetrieverFunc().StartTime() }
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider := NewCManaWeightProvider(manaRetrieverFunc, timeRetrieverFunc, confirmedRetrieverFunc)

	// Add node1 as active in the genesis epoch.
	{
		weightProvider.Update(epochRetrieverFunc(), nodes["1"])
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
		})
	}

	// Add node2 and node3 activity at epoch == 2 -> only node1 is active.
	{
		weightProvider.Update(epochRetrieverFunc()+1, nodes["2"])
		weightProvider.Update(epochRetrieverFunc()+1, nodes["3"])
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
		})
	}

	// Advance LatestCommittableEpoch by one epoch -> all nodes are active.
	{

		epochManager.ei = epochRetrieverFunc() + 1
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
			"2": 50,
			"3": 30,
		})
	}

	// Advance LatestCommittableEpoch by two epochs -> node1 and node2 are active.
	{
		epochManager.ei = epochRetrieverFunc() + activeEpochThreshold
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"2": 50,
			"3": 30,
		})
	}

	// Advance LatestCommittableEpoch by three epochs -> no node is active anymore.
	{
		epochManager.ei = epochRetrieverFunc() + 2
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{})
	}
}

func assertWeightsOfRelevantVoters(t *testing.T, weightProvider WeightProvider, nodes map[string]identity.ID, expectedAlias map[string]float64) {
	expected := make(map[identity.ID]float64)
	var expectedTotalWeight float64

	for alias, weight := range expectedAlias {
		expected[nodes[alias]] = weight
		expectedTotalWeight += weight
	}

	weights, totalWeight := weightProvider.WeightsOfRelevantVoters()
	assert.EqualValues(t, expected, weights)
	assert.Equal(t, expectedTotalWeight, totalWeight)
}
