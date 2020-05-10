package tipmanager

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/stretchr/testify/assert"
)

func TestTipManagerSimple(t *testing.T) {
	tipManager := New()

	// check if first tips point to genesis
	parent1, parent2 := tipManager.Tips()
	assert.Equal(t, payload.GenesisID, parent1)
	assert.Equal(t, payload.GenesisID, parent2)

	// add few tips and check whether total tips count matches
	totalTips := 100
	branchesMap := make(map[branchmanager.BranchID]*payload.Payload, totalTips)
	branches := make([]branchmanager.BranchID, totalTips)
	for i := 0; i < totalTips; i++ {
		branchID := branchmanager.NewBranchID(transaction.RandomID())
		branches[i] = branchID
		branchesMap[branchID] = payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		tipManager.AddTip(branchesMap[branchID], branchID)
	}
	assert.Equal(t, totalTips, tipManager.TipCount(branches...))

	// check if tips point to genesis when branches are not found
	parent1, parent2 = tipManager.Tips(branchmanager.UndefinedBranchID, branchmanager.NewBranchID(transaction.RandomID()))
	assert.Equal(t, 0, tipManager.TipCount(branchmanager.UndefinedBranchID, branchmanager.NewBranchID(transaction.RandomID())))
	assert.Equal(t, payload.GenesisID, parent1)
	assert.Equal(t, payload.GenesisID, parent2)

	// use each branch to get the single tip
	for b, o := range branchesMap {
		parent1ObjectID, parent2ObjectID := tipManager.Tips(b)
		assert.Equal(t, o.ID(), parent1ObjectID)
		assert.Equal(t, o.ID(), parent2ObjectID)
	}

	// select tips from two branches
	parent1, parent2 = tipManager.Tips(branches[0], branches[1])
	s := []payload.ID{
		branchesMap[branches[0]].ID(),
		branchesMap[branches[1]].ID(),
	}
	assert.Contains(t, s, parent1)
	assert.Contains(t, s, parent2)
}

func TestTipManagerParallel(t *testing.T) {
	totalTips := 1000
	totalThreads := 100
	totalBranches := 10

	tipManager := New()

	branches := make([]branchmanager.BranchID, totalBranches)
	for i := 0; i < totalBranches; i++ {
		branchID := branchmanager.NewBranchID(transaction.RandomID())
		branches[i] = branchID
	}

	// add tips in parallel
	var wg sync.WaitGroup
	for i := 0; i < totalThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for t := 0; t < totalTips; t++ {
				tip := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
				random := rand.Intn(totalBranches)
				tipManager.AddTip(tip, branches[random])
			}
		}()
	}
	wg.Wait()

	// check total tip count
	assert.Equal(t, totalTips*totalThreads, tipManager.TipCount(branches...))
}

func TestTipManager_weightedRandom(t *testing.T) {
	totalRounds := 1000000
	weights := []int{1, 20, 39, 30, 9, 1}
	weightsSum := 0
	for _, w := range weights {
		weightsSum += w
	}
	// make sure result is within delta of 0.01
	delta := float64(totalRounds / weightsSum)

	counts := make([]int, len(weights))

	// calculate weighted random
	for i := 0; i < totalRounds; i++ {
		counts[weightedRandom(weights)]++
	}

	for i, c := range counts {
		expected := totalRounds * weights[i] / weightsSum
		assert.InDelta(t, expected, c, delta)
	}
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
