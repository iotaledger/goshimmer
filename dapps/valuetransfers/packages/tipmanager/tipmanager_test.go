package tipmanager

import (
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

	// add 1 transaction in liked branch -> both tips need to point there
	tx := createDummyTransaction()
	branch1 := newMockBranch(tx.ID(), true)
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	branch1.addTip(valueObject)
	tipManager.AddTip(valueObject, branch1)

	assert.Equal(t, 1, tipManager.TipCount())
	parent1, parent2 = tipManager.Tips()
	assert.Equal(t, valueObject.ID(), parent1)
	assert.Equal(t, valueObject.ID(), parent2)

	// add a bunch of liked tips -> check count
	numTips := 10
	branch2 := newMockBranch(transaction.RandomID(), true)
	for i := 0; i < numTips; i++ {
		v := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		branch1.addTip(v)
		tipManager.AddTip(v, branch1)

		v = payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		branch2.addTip(v)
		tipManager.AddTip(v, branch2)
	}

	assert.Equal(t, branch1.tipCount()+branch2.tipCount(), tipManager.TipCount())
	assert.Equal(t, branch1.tipCount(), len(tipManager.branches[branch1.ID()].tips))
	assert.Equal(t, branch2.tipCount(), len(tipManager.branches[branch2.ID()].tips))
	parent1, parent2 = tipManager.Tips()
	assert.NotEqual(t, parent1, parent2)

	// add a bunch of disliked tips -> count should be unchanged
	branch3 := newMockBranch(transaction.RandomID(), false)
	branch4 := newMockBranch(transaction.RandomID(), false)
	for i := 0; i < numTips; i++ {
		v := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		branch3.addTip(v)
		tipManager.AddTip(v, branch3)

		v = payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		branch4.addTip(v)
		tipManager.AddTip(v, branch4)
	}

	assert.Equal(t, branch1.tipCount()+branch2.tipCount(), tipManager.TipCount())
	assert.Equal(t, branch1.tipCount(), len(tipManager.branches[branch1.ID()].tips))
	assert.Equal(t, branch2.tipCount(), len(tipManager.branches[branch2.ID()].tips))
	assert.Equal(t, branch3.tipCount(), len(tipManager.branches[branch3.ID()].tips))
	assert.Equal(t, branch4.tipCount(), len(tipManager.branches[branch4.ID()].tips))

	// add tip to liked branch with parents from same branch -> should be in tipmanager.tips and parents removed from tipmanager.tips and branch's tips
	removedTip1 := branch1.tip()
	removedTip2 := branch1.tip()
	valueObject = payload.New(removedTip1, removedTip2, createDummyTransaction())
	branch1.addTip(valueObject)
	tipManager.AddTip(valueObject, branch1)

	assert.Equal(t, branch1.tipCount()+branch2.tipCount(), tipManager.TipCount())
	_, ok := tipManager.tips.Get(removedTip1)
	assert.False(t, ok)
	_, ok = tipManager.tips.Get(removedTip2)
	assert.False(t, ok)
	_, ok = tipManager.tips.Get(valueObject.ID())
	assert.True(t, ok)

	_, ok = tipManager.branches[branch1.ID()].tips[removedTip1]
	assert.False(t, ok)
	_, ok = tipManager.branches[branch1.ID()].tips[removedTip2]
	assert.False(t, ok)
	_, ok = tipManager.branches[branch1.ID()].tips[valueObject.ID()]
	assert.True(t, ok)

	// add tip to disliked branch with parents from same branch -> tipmanager.tips should be unmodified, only branch's tips modified
	removedTip1 = branch3.tip()
	removedTip2 = branch3.tip()
	valueObject = payload.New(removedTip1, removedTip2, createDummyTransaction())
	branch3.addTip(valueObject)
	tipManager.AddTip(valueObject, branch3)

	assert.Equal(t, branch1.tipCount()+branch2.tipCount(), tipManager.TipCount())
	_, ok = tipManager.branches[branch3.ID()].tips[removedTip1]
	assert.False(t, ok)
	_, ok = tipManager.branches[branch3.ID()].tips[removedTip2]
	assert.False(t, ok)
	_, ok = tipManager.branches[branch3.ID()].tips[valueObject.ID()]
	assert.True(t, ok)
}

// TestTipManagerParallel tests the TipManager's functionality by adding and selecting tips concurrently.
func TestTipManagerConcurrent(t *testing.T) {
	numThreads := 10
	numTips := 100
	branches := make([]*mockBranch, numThreads)

	tipManager := New()
	var wg sync.WaitGroup

	// add tips to numThreads branches in parallel
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func(number int) {
			defer wg.Done()

			branch := newMockBranch(transaction.RandomID(), number < numThreads/2)
			branches[number] = branch

			// add tips within a branch in parallel
			var wg2 sync.WaitGroup
			for t := 0; t < numTips; t++ {
				v := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
				branch.addTip(v)
				wg2.Add(1)
				go func() {
					defer wg2.Done()
					tipManager.AddTip(v, branch)
				}()
			}
			wg2.Wait()

			removedTip1 := branch.tip()
			removedTip2 := branch.tip()
			valueObject := payload.New(removedTip1, removedTip2, createDummyTransaction())
			branch.addTip(valueObject)
			tipManager.AddTip(valueObject, branch)
		}(i)
	}
	wg.Wait()

	// verify tips of every branch
	for i := 0; i < numThreads; i++ {
		branch := branches[i]
		assert.Equal(t, branch.tipCount(), len(tipManager.branches[branch.ID()].tips))

		for tip := range branch.tips {
			// all tips should be in branch's list
			_, ok := tipManager.branches[branch.ID()].tips[tip]
			assert.True(t, ok)
			// make sure that all tips from liked branches (and none from disliked branches) are in tipmanager.tips
			_, ok = tipManager.tips.Get(tip)
			assert.Equal(t, branch.Liked(), ok)
		}
	}

	// total tips: numThreads/2 (liked branches) * numTips - numThreads/2 (selected tips)
	assert.Equal(t, numThreads/2*numTips-numThreads/2, tipManager.TipCount())
}

type mockBranch struct {
	id    branchmanager.BranchID
	liked bool
	tips  map[payload.ID]payload.ID
}

func newMockBranch(txID transaction.ID, liked bool) *mockBranch {
	return &mockBranch{
		id:    branchmanager.NewBranchID(txID),
		liked: liked,
		tips:  make(map[payload.ID]payload.ID),
	}
}

func (m *mockBranch) ID() branchmanager.BranchID {
	return m.id
}
func (m *mockBranch) Liked() bool {
	return m.liked
}

func (m *mockBranch) addTip(valueObject *payload.Payload) {
	m.tips[valueObject.ID()] = valueObject.ID()
}

func (m *mockBranch) tip() payload.ID {
	var tip payload.ID

	// get some element
	for t := range m.tips {
		tip = t
	}
	delete(m.tips, tip)

	return tip
}

func (m *mockBranch) tipCount() int {
	return len(m.tips)
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
