package tangle

import (
	"github.com/iotaledger/hive.go/datastructure/randommap"
)

// MessageTipSelector manages a map of tips and emits events for their removal and addition.
type MessageTipSelector struct {
	tips   *randommap.RandomMap
	Events *MessageTipSelectorEvents
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
	for _, messageID := range tips {
		t.tips.Set(messageID, messageID)
	}
}

// AddTip adds the given message as a tip.
func (t *MessageTipSelector) AddTip(msg *Message) {
	messageID := msg.ID()
	if t.tips.Set(messageID, messageID) {
		t.Events.TipAdded.Trigger(messageID)
	}

	parent1MessageID := msg.Parent1ID()
	if _, deleted := t.tips.Delete(parent1MessageID); deleted {
		t.Events.TipRemoved.Trigger(parent1MessageID)
	}

	parent2MessageID := msg.Parent2ID()
	if _, deleted := t.tips.Delete(parent2MessageID); deleted {
		t.Events.TipRemoved.Trigger(parent2MessageID)
	}
}

// Tips returns two tips.
func (t *MessageTipSelector) Tips() (parent1MessageID, parent2MessageID MessageID) {
	tip := t.tips.RandomEntry()
	if tip == nil {
		parent1MessageID = EmptyMessageID
		parent2MessageID = EmptyMessageID

		return
	}

	parent2MessageID = tip.(MessageID)

	if t.tips.Size() == 1 {
		parent1MessageID = parent2MessageID
		return
	}

	parent1MessageID = t.tips.RandomEntry().(MessageID)
	for parent1MessageID == parent2MessageID && t.tips.Size() > 1 {
		parent1MessageID = t.tips.RandomEntry().(MessageID)
	}

	return
}

// TipCount the amount of current tips.
func (t *MessageTipSelector) TipCount() int {
	return t.tips.Size()
}
