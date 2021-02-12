package tangle

import (
	"github.com/iotaledger/hive.go/datastructure/randommap"
	"github.com/iotaledger/hive.go/events"
)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

// TipManager manages a map of tips and emits events for their removal and addition.
type TipManager struct {
	tangle *Tangle
	tips   *randommap.RandomMap
	Events *TipManagerEvents
}

// NewTipManager creates a new tip-selector.
func NewTipManager(tangle *Tangle, tips ...MessageID) *TipManager {
	tipSelector := &TipManager{
		tangle: tangle,
		tips:   randommap.New(),
		Events: &TipManagerEvents{
			TipAdded:   events.NewEvent(messageIDEventHandler),
			TipRemoved: events.NewEvent(messageIDEventHandler),
		},
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TipManager) Setup() {
	t.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		t.tangle.Storage.Message(messageID).Consume(t.AddTip)
	}))
}

// Set adds the given messageIDs as tips.
func (t *TipManager) Set(tips ...MessageID) {
	for _, messageID := range tips {
		t.tips.Set(messageID, messageID)
	}
}

// AddTip adds the given message as a tip.
func (t *TipManager) AddTip(msg *Message) {
	messageID := msg.ID()
	if t.tips.Set(messageID, messageID) {
		t.Events.TipAdded.Trigger(messageID)
	}

	msg.ForEachStrongParent(func(parent MessageID) {
		if _, deleted := t.tips.Delete(parent); deleted {
			t.Events.TipRemoved.Trigger(parent)
		}
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(count int) (parents []MessageID) {
	if count > MaxParentsCount {
		count = MaxParentsCount
	}
	if count < MinParentsCount {
		count = MinParentsCount
	}
	parents = make([]MessageID, 0, count)

	tips := t.tips.RandomUniqueEntries(count)
	// count is not valid
	if tips == nil {
		parents = append(parents, EmptyMessageID)
		return
	}
	// count is valid, but there simply are no tips
	if len(tips) == 0 {
		parents = append(parents, EmptyMessageID)
		return
	}
	// at least one tip is returned
	for _, tip := range tips {
		parents = append(parents, tip.(MessageID))
	}

	return
}

// TipCount the amount of current tips.
func (t *TipManager) TipCount() int {
	return t.tips.Size()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipManagerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// TipManagerEvents represents events happening on the TipManager.
type TipManagerEvents struct {
	// Fired when a tip is added.
	TipAdded *events.Event

	// Fired when a tip is removed.
	TipRemoved *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
