package tangle

import (
	"sync"

	"github.com/iotaledger/hive.go/datastructure/randommap"
)

// MessageTipSelector manages a map of tips and emits events for their removal and addition.
type MessageTipSelector struct {
	tips   *randommap.RandomMap
	Events *MessageTipSelectorEvents
	mutex  sync.RWMutex
}

// NewMessageTipSelector creates a new tip-selector.
func NewMessageTipSelector(tips ...MessageID) *MessageTipSelector {
	tipSelector := &MessageTipSelector{
		tips:   randommap.New(),
		Events: newMessageTipSelectorEvents(),
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Set adds the given messageIDs as tips.
func (t *MessageTipSelector) Set(tips ...MessageID) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.set(tips...)
}

// set adds the given messageIDs as tips.
func (t *MessageTipSelector) set(tips ...MessageID) {
	for _, messageID := range tips {
		t.tips.Set(messageID, messageID)
	}
}

// AddTip adds the given message as a tip.
func (t *MessageTipSelector) AddTip(msg *Message) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.addTip(msg)
}

// addTip adds the given message as a tip.
func (t *MessageTipSelector) addTip(msg *Message) {
	messageID := msg.ID()
	if t.tips.Set(messageID, messageID) {

		msg.ForEachStrongParent(func(parent MessageID) {
			if _, deleted := t.tips.Delete(parent); deleted {
				t.Events.TipRemoved.Trigger(parent)
			}
		})

		t.Events.TipAdded.Trigger(messageID)
	}
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *MessageTipSelector) Tips(count int) (parents []MessageID) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.getTips(count)
}

// getTips returns count number of tips, maximum MaxParentsCount.
func (t *MessageTipSelector) getTips(count int) (parents []MessageID) {
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
func (t *MessageTipSelector) TipCount() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.tipCount()
}

// tipCount the amount of current tips.
func (t *MessageTipSelector) tipCount() int {
	return t.tips.Size()
}

// AllTips return all the current tips.
func (t *MessageTipSelector) AllTips() (tips []MessageID) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.allTips()
}

func (t *MessageTipSelector) allTips() (tips []MessageID) {
	for _, tip := range t.tips.Keys() {
		tips = append(tips, tip.(MessageID))
	}
	return
}
