package tangle

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
)

// MessageTipSelector manages a map of tips and emits events for their removal and addition.
type MessageTipSelector struct {
	tips   *datastructure.RandomMap
	Events *MessageTipSelectorEvents
}

// NewMessageTipSelector creates a new tip-selector.
func NewMessageTipSelector(tips ...MessageID) *MessageTipSelector {
	tipSelector := &MessageTipSelector{
		tips:   datastructure.NewRandomMap(),
		Events: newMessageTipSelectorEvents(),
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Set adds the given messageIDs as tips.
func (tipSelector *MessageTipSelector) Set(tips ...MessageID) {
	for _, messageID := range tips {
		tipSelector.tips.Set(messageID, messageID)
	}
}

// AddTip adds the given message as a tip.
func (tipSelector *MessageTipSelector) AddTip(msg *Message) {
	messageID := msg.ID()
	if tipSelector.tips.Set(messageID, messageID) {
		tipSelector.Events.TipAdded.Trigger(messageID)
	}

	parent1MessageID := msg.Parent1ID()
	if _, deleted := tipSelector.tips.Delete(parent1MessageID); deleted {
		tipSelector.Events.TipRemoved.Trigger(parent1MessageID)
	}

	parent2MessageID := msg.Parent2ID()
	if _, deleted := tipSelector.tips.Delete(parent2MessageID); deleted {
		tipSelector.Events.TipRemoved.Trigger(parent2MessageID)
	}
}

// Tips returns two tips.
func (tipSelector *MessageTipSelector) Tips() (parent1MessageID, parent2MessageID MessageID) {
	tip := tipSelector.tips.RandomEntry()
	if tip == nil {
		parent1MessageID = EmptyMessageID
		parent2MessageID = EmptyMessageID

		return
	}

	parent2MessageID = tip.(MessageID)

	if tipSelector.tips.Size() == 1 {
		parent1MessageID = parent2MessageID
		return
	}

	parent1MessageID = tipSelector.tips.RandomEntry().(MessageID)
	for parent1MessageID == parent2MessageID && tipSelector.tips.Size() > 1 {
		parent1MessageID = tipSelector.tips.RandomEntry().(MessageID)
	}

	return
}

// TipCount the amount of current tips.
func (tipSelector *MessageTipSelector) TipCount() int {
	return tipSelector.tips.Size()
}
