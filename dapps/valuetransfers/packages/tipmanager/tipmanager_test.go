package tipmanager

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

// TestTipManager tests the functionality of the TipManager.
func TestTipManager(t *testing.T) {
	tipManager := New()

	// check if first tips point to genesis
	parent1, parent2 := tipManager.Tips()
	assert.Equal(t, payload.GenesisID, parent1)
	assert.Equal(t, payload.GenesisID, parent2)

	// create value object and add it
	v := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
	tipManager.AddTip(v)

	// check count
	assert.Equal(t, 1, tipManager.TipCount())

	// check if both reference it
	parent1, parent2 = tipManager.Tips()
	assert.Equal(t, v.ID(), parent1)
	assert.Equal(t, v.ID(), parent2)

	// create value object and add it
	v2 := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
	tipManager.AddTip(v2)

	// check count
	assert.Equal(t, 2, tipManager.TipCount())

	// attach new value object to previous 2 tips
	parent1, parent2 = tipManager.Tips()
	assert.Contains(t, []payload.ID{v.ID(), v2.ID()}, parent1)
	assert.Contains(t, []payload.ID{v.ID(), v2.ID()}, parent2)
	v3 := payload.New(parent1, parent2, createDummyTransaction())
	tipManager.AddTip(v3)

	// check that parents are removed
	assert.Equal(t, 1, tipManager.TipCount())
	parent1, parent2 = tipManager.Tips()
	assert.Equal(t, v3.ID(), parent1)
	assert.Equal(t, v3.ID(), parent2)
}

// TestTipManagerParallel tests the TipManager's functionality by adding and selecting tips concurrently.
func TestTipManagerConcurrent(t *testing.T) {
	numThreads := 10
	numTips := 100
	numSelected := 10

	var tipsAdded uint64
	countTipAdded := events.NewClosure(func(valueObjectID payload.ID) {
		atomic.AddUint64(&tipsAdded, 1)
	})
	var tipsRemoved uint64
	countTipRemoved := events.NewClosure(func(valueObjectID payload.ID) {
		atomic.AddUint64(&tipsRemoved, 1)
	})

	var wg sync.WaitGroup
	tipManager := New()
	tipManager.Events.TipAdded.Attach(countTipAdded)
	tipManager.Events.TipRemoved.Attach(countTipRemoved)

	for t := 0; t < numThreads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			tips := make(map[payload.ID]struct{})
			// add a bunch of tips
			for i := 0; i < numTips; i++ {
				v := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
				tipManager.AddTip(v)
				tips[v.ID()] = struct{}{}
			}
			// add a bunch of tips that reference previous tips
			for i := 0; i < numSelected; i++ {
				v := payload.New(randomTip(tips), randomTip(tips), createDummyTransaction())
				tipManager.AddTip(v)
			}
		}()
	}

	wg.Wait()

	// check if count matches and corresponding events have been triggered
	assert.EqualValues(t, numTips*numThreads+numSelected*numThreads, tipsAdded)
	assert.EqualValues(t, 2*numSelected*numThreads, tipsRemoved)
	assert.EqualValues(t, numTips*numThreads-numSelected*numThreads, tipManager.TipCount())
}

func randomTip(tips map[payload.ID]struct{}) payload.ID {
	var tip payload.ID

	for k := range tips {
		tip = k
	}
	delete(tips, tip)

	return tip
}

func createDummyTransaction() *transaction.Transaction {
	return transaction.New(
		// inputs
		transaction.NewInputs(
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
		),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	)
}
