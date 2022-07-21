package tangleold

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
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

	activeNodes := make(map[identity.ID]*epoch.ActivityLog)
	for _, nodeID := range nodes {
		a := epoch.NewActivityLog()

		for i := 0; i < crypto.Randomness.Intn(100); i++ {
			a.Add(epoch.Index(i))
		}

		activeNodes[nodeID] = a
	}
	activeNodesBytes := activeNodesToBytes(activeNodes)
	activeNodes2, err := activeNodesFromBytes(activeNodesBytes)
	require.NoError(t, err)

	for nodeID, a := range activeNodes {
		assert.EqualValues(t, a.SetEpochs.Size(), activeNodes2[nodeID].SetEpochs.Size())
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

	startEpoch := epoch.IndexFromTime(time.Now())
	epochManager := &struct {
		ei epoch.Index
	}{
		ei: startEpoch,
	}
	epochRetrieverFunc := func() epoch.Index { return epochManager.ei }
	timeRetrieverFunc := func() time.Time { return epochRetrieverFunc().StartTime() }
	weightProvider := NewCManaWeightProvider(manaRetrieverFunc, timeRetrieverFunc)

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
		epochManager.ei = epochRetrieverFunc() + 2
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
