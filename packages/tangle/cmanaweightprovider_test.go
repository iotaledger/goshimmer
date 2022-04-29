package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveNodesMarshalling(t *testing.T) {
	nodes := map[string]identity.ID{
		"node1": identity.GenerateIdentity().ID(),
		"node2": identity.GenerateIdentity().ID(),
		"node3": identity.GenerateIdentity().ID(),
	}

	activeNodes := make(map[identity.ID]*ActivityLog)
	for _, nodeID := range nodes {
		a := NewActivityLog()

		for i := 0; i < crypto.Randomness.Intn(100); i++ {
			a.Add(time.Now().Add(time.Duration(i)*time.Minute + time.Hour))
		}

		activeNodes[nodeID] = a
	}
	activeNodesBytes := activeNodesToBytes(activeNodes)
	activeNodes2, err := activeNodesFromBytes(activeNodesBytes)
	require.NoError(t, err)

	for nodeID, a := range activeNodes {
		assert.EqualValues(t, a.setTimes.Size(), activeNodes2[nodeID].setTimes.Size())
	}
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

	tangleTime := time.Unix(DefaultGenesisTime, 0)
	timeRetrieverFunc := func() time.Time { return tangleTime }

	weightProvider := NewCManaWeightProvider(manaRetrieverFunc, timeRetrieverFunc)

	// Add node1 as active in the genesis.
	{
		weightProvider.Update(tangleTime, nodes["1"])
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
		})
	}

	// Add node2 and node3 activity at tangleTime+20 -> only node1 is active.
	{
		weightProvider.Update(tangleTime.Add(20*time.Minute), nodes["2"])
		weightProvider.Update(tangleTime.Add(20*time.Minute), nodes["3"])
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
		})
	}

	// Advance TangleTime by 25min -> all nodes are active.
	{
		tangleTime = tangleTime.Add(25 * time.Minute)
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"1": 20,
			"2": 50,
			"3": 30,
		})
	}

	// Advance TangleTime by 10min -> node1 and node2 are active.
	{
		tangleTime = tangleTime.Add(25 * time.Minute)
		assertWeightsOfRelevantVoters(t, weightProvider, nodes, map[string]float64{
			"2": 50,
			"3": 30,
		})
	}

	// Advance tangleTime by 25min -> no node is active anymore.
	{
		tangleTime = tangleTime.Add(25 * time.Minute)
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
