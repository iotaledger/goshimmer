package epochs

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_EpochMarshaling(t *testing.T) {
	epoch := NewEpoch(1)
	id, _ := identity.RandomID()
	epoch.AddNode(id)

	epochFromBytes, _, err := EpochFromBytes(epoch.Bytes())
	require.NoError(t, err)

	assert.Equal(t, epoch.Bytes(), epochFromBytes.Bytes())
	assert.Equal(t, epoch.TotalMana(), epochFromBytes.TotalMana())
	assert.Equal(t, epoch.mana[id], epochFromBytes.mana[id])
}

func TestEpochs(t *testing.T) {
	nodes := make(map[string]identity.ID)
	for _, node := range []string{"A", "B", "C", "D"} {
		nodes[node] = identity.GenerateIdentity().ID()
	}

	manaEpoch0 := map[identity.ID]float64{
		nodes["A"]: 100,
	}
	manaAllEpochs := map[identity.ID]float64{
		nodes["A"]: 100,
		nodes["B"]: 25,
		nodes["C"]: 33,
		nodes["D"]: 89,
	}

	var manager *Manager
	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		epochID := manager.TimeToEpochID(t)

		if epochID < 2 {
			return manaEpoch0
		}

		return manaAllEpochs
	}

	manager = NewManager(ManaRetriever(manaRetrieverMock), CacheTime(0))

	// Add messages to epoch 2
	{
		epochID := ID(2)

		manager.Update(manager.randomTimeInEpoch(epochID), nodes["A"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["A"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["B"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["C"])

		assert.Truef(t, manager.Epoch(epochID).Consume(func(epoch *Epoch) {
			for _, name := range []string{"A", "B", "C"} {
				_, exists := epoch.Mana()[nodes[name]]
				assert.True(t, exists)
			}
		}), "%s was not found in storage", epochID)
	}

	// Add messages to epoch 3
	{
		epochID := ID(3)

		manager.Update(manager.randomTimeInEpoch(epochID), nodes["A"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["B"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["D"])

		assert.Truef(t, manager.Epoch(epochID).Consume(func(epoch *Epoch) {
			for _, name := range []string{"A", "B", "D"} {
				_, exists := epoch.Mana()[nodes[name]]
				assert.True(t, exists)
			}
		}), "%s was not found in storage", epochID)
	}

	// Add messages to epoch 4
	{
		epochID := ID(4)

		manager.Update(manager.randomTimeInEpoch(epochID), nodes["A"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["B"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["C"])
		manager.Update(manager.randomTimeInEpoch(epochID), nodes["D"])

		assert.Truef(t, manager.Epoch(epochID).Consume(func(epoch *Epoch) {
			for _, name := range []string{"A", "B", "C", "D"} {
				_, exists := epoch.Mana()[nodes[name]]
				assert.True(t, exists)
			}
		}), "%s was not found in storage", epochID)
	}

	// Retrieve active mana in epoch 3 -> oracle epoch 0
	{
		epochID := ID(3)
		activeMana, totalMana := manager.ActiveMana(manager.TimeToOracleEpochID(manager.randomTimeInEpoch(epochID)))

		assert.Len(t, activeMana, 1)
		assert.EqualValues(t, 100, totalMana)
		assert.Contains(t, activeMana, nodes["A"])
		assert.NotContains(t, activeMana, nodes["B"])
		assert.NotContains(t, activeMana, nodes["C"])
		assert.NotContains(t, activeMana, nodes["D"])
	}

	// Retrieve active mana in epoch 4 -> oracle epoch 2
	{
		epochID := ID(4)
		activeMana, totalMana := manager.ActiveMana(manager.TimeToOracleEpochID(manager.randomTimeInEpoch(epochID)))

		assert.Len(t, activeMana, 3)
		assert.EqualValues(t, 158, totalMana)
		assert.Contains(t, activeMana, nodes["A"])
		assert.Contains(t, activeMana, nodes["B"])
		assert.Contains(t, activeMana, nodes["C"])
		assert.NotContains(t, activeMana, nodes["D"])
	}

	// Retrieve active mana in epoch 5 -> oracle epoch 3
	{
		epochID := ID(5)
		activeMana, totalMana := manager.ActiveMana(manager.TimeToOracleEpochID(manager.randomTimeInEpoch(epochID)))

		assert.Len(t, activeMana, 3)
		assert.EqualValues(t, 214, totalMana)
		assert.Contains(t, activeMana, nodes["A"])
		assert.Contains(t, activeMana, nodes["B"])
		assert.NotContains(t, activeMana, nodes["C"])
		assert.Contains(t, activeMana, nodes["D"])
	}

	// Retrieve active mana in epoch 6 -> oracle epoch 4
	{
		epochID := ID(6)
		activeMana, totalMana := manager.ActiveMana(manager.TimeToOracleEpochID(manager.randomTimeInEpoch(epochID)))

		assert.Len(t, activeMana, 4)
		assert.EqualValues(t, 247, totalMana)
		assert.Contains(t, activeMana, nodes["A"])
		assert.Contains(t, activeMana, nodes["B"])
		assert.Contains(t, activeMana, nodes["C"])
		assert.Contains(t, activeMana, nodes["D"])
	}

	// TODO: what do do in this case? we don't have any entries yet for epoch 5
	// Retrieve active mana in epoch 7 -> oracle epoch 5
	{
	}
}
