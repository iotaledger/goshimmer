package tipmanager

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/hive.go/events"
)

// TipManager manages liked tips and emits events for their removal and addition.
type TipManager struct {
	// tips are all currently liked tips.
	tips   *datastructure.RandomMap
	Events Events
}

// New creates a new TipManager.
func New() *TipManager {
	return &TipManager{
		tips: datastructure.NewRandomMap(),
		Events: Events{
			TipAdded:   events.NewEvent(payloadIDEvent),
			TipRemoved: events.NewEvent(payloadIDEvent),
		},
	}
}

// AddTip adds the given value object as a tip.
func (t *TipManager) AddTip(valueObject *payload.Payload) {
	objectID := valueObject.ID()
	parent1ID := valueObject.TrunkID()
	parent2ID := valueObject.BranchID()

	if t.tips.Set(objectID, objectID) {
		t.Events.TipAdded.Trigger(objectID)
	}

	// remove parents
	if _, deleted := t.tips.Delete(parent1ID); deleted {
		t.Events.TipRemoved.Trigger(parent1ID)
	}
	if _, deleted := t.tips.Delete(parent2ID); deleted {
		t.Events.TipRemoved.Trigger(parent2ID)
	}
}

// RemoveTip removes the given value object as a tip.
func (t *TipManager) RemoveTip(valueObject *payload.Payload) {
	objectID := valueObject.ID()

	if _, deleted := t.tips.Delete(objectID); deleted {
		t.Events.TipRemoved.Trigger(objectID)
	}
}

// Tips returns two randomly selected tips.
func (t *TipManager) Tips() (parent1ObjectID, parent2ObjectID payload.ID) {
	tip := t.tips.RandomEntry()
	if tip == nil {
		parent1ObjectID = payload.GenesisID
		parent2ObjectID = payload.GenesisID
		return
	}

	parent1ObjectID = tip.(payload.ID)

	if t.tips.Size() == 1 {
		parent2ObjectID = parent1ObjectID
		return
	}

	parent2ObjectID = t.tips.RandomEntry().(payload.ID)
	// select 2 distinct tips if there's more than 1 tip available
	for parent1ObjectID == parent2ObjectID && t.tips.Size() > 1 {
		parent2ObjectID = t.tips.RandomEntry().(payload.ID)
	}

	return
}

// AllTips returns all the tips.
func (t *TipManager) AllTips() (tips []payload.ID) {
	tips = make([]payload.ID, t.Size())
	for i, tip := range t.tips.Keys() {
		tips[i] = tip.(payload.ID)
	}
	return
}

// Size returns the total liked tips.
func (t *TipManager) Size() int {
	return t.tips.Size()
}
