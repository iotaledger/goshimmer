package tipmanager

import (
	"math/rand"
	"sync"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/hive.go/events"
)

// TipManager manages tips of all branches and emits events for their removal and addition.
type TipManager struct {
	tips      map[branchmanager.BranchID]*datastructure.RandomMap
	tipsMutex sync.Mutex
	Events    Events
}

// New creates a new TipManager.
func New() *TipManager {
	return &TipManager{
		tips: make(map[branchmanager.BranchID]*datastructure.RandomMap),
		Events: Events{
			TipAdded:   events.NewEvent(payloadIDEvent),
			TipRemoved: events.NewEvent(payloadIDEvent),
		},
	}
}

// AddTip adds the given value object as a tip in the given branch.
func (t *TipManager) AddTip(valueObject *payload.Payload, branch branchmanager.BranchID) {
	objectID := valueObject.ID()
	//parent1ID := valueObject.TrunkID()
	//parent2ID := valueObject.BranchID()

	t.tipsMutex.Lock()
	defer t.tipsMutex.Unlock()

	branchTips, ok := t.tips[branch]
	if !ok {
		// add new branch to tips map
		branchTips = datastructure.NewRandomMap()
		t.tips[branch] = branchTips
	}

	if branchTips.Set(objectID, objectID) {
		t.Events.TipAdded.Trigger(objectID, branch)
	}

	// remove parents
	// TODO: parents could be in another branch. get branch via ID ?
	// utxoDAG.ValueObjectBooking(valueObjectID).Unwrap().BranchID()
	//if _, deleted := branchTips.Delete(parent1ID); deleted {
	//	t.Events.TipRemoved.Trigger(parent1ID, branch)
	//}
	//
	//if _, deleted := branchTips.Delete(parent2ID); deleted {
	//	t.Events.TipRemoved.Trigger(parent2ID, branch)
	//}
}

// Tips randomly selects tips in the given branches weighted by size.
func (t *TipManager) Tips(branches ...branchmanager.BranchID) (parent1ObjectID, parent2ObjectID payload.ID) {
	if len(branches) == 0 {
		parent1ObjectID = payload.GenesisID
		parent2ObjectID = payload.GenesisID
		return
	}

	weights := make([]int, len(branches))
	totalTips := 0
	tipsSlice := make([]*datastructure.RandomMap, len(branches))

	t.tipsMutex.Lock()
	defer t.tipsMutex.Unlock()

	// prepare weighted random selection
	for i, b := range branches {
		branchTips, ok := t.tips[b]
		if !ok {
			continue
		}

		tipsSlice[i] = branchTips
		weights[i] = branchTips.Size()
		totalTips += weights[i]
	}

	// no tips in the given branches
	// TODO: what exactly to do in this case?
	if totalTips == 0 {
		parent1ObjectID = payload.GenesisID
		parent2ObjectID = payload.GenesisID
		return
	}

	// select tips
	random := weightedRandom(weights)

	branchTips := tipsSlice[random]
	tip := branchTips.RandomEntry()
	if tip == nil {
		// this case should never occur due to weighted random selection
		panic("tip is nil.")
	}
	parent1ObjectID = tip.(payload.ID)

	if totalTips == 1 {
		parent2ObjectID = parent1ObjectID
		return
	}

	// adjust weights
	weights[random] -= 1

	branchTips = tipsSlice[weightedRandom(weights)]
	tip = branchTips.RandomEntry()
	if tip == nil {
		// this case should never occur due to weighted random selection
		panic("tip is nil.")
	}
	parent2ObjectID = tip.(payload.ID)

	return
}

// TipCount returns the total tips in the given branches.
func (t *TipManager) TipCount(branches ...branchmanager.BranchID) int {
	t.tipsMutex.Lock()
	defer t.tipsMutex.Unlock()

	totalTips := 0
	for _, b := range branches {
		branchTips, ok := t.tips[b]
		if !ok {
			continue
		}

		totalTips += branchTips.Size()
	}
	return totalTips
}

func weightedRandom(weights []int) int {
	if len(weights) == 0 {
		return 0
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}
	r := rand.Intn(totalWeight)

	for i, w := range weights {
		r -= w
		if r < 0 {
			return i
		}
	}
	return len(weights) - 1
}
