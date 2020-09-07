package tipselector

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// TipSelector manages a map of tips and emits events for their removal and addition.
type TipSelector struct {
	tips   *datastructure.RandomMap
	Events *Events
}

// New creates a new tip-selector.
func New(tips ...message.ID) *TipSelector {
	tipSelector := &TipSelector{
		tips:   datastructure.NewRandomMap(),
		Events: newEvents(),
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Set adds the given messageIDs as tips.
func (tipSelector *TipSelector) Set(tips ...message.ID) {
	for _, messageID := range tips {
		tipSelector.tips.Set(messageID, messageID)
	}
}

// AddTip adds the given message as a tip.
func (tipSelector *TipSelector) AddTip(msg *message.Message) {
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
func (tipSelector *TipSelector) Tips() (parent1MessageID, parent2MessageID message.ID) {
	tip := tipSelector.tips.RandomEntry()
	if tip == nil {
		parent1MessageID = message.EmptyID
		parent2MessageID = message.EmptyID

		return
	}

	parent2MessageID = tip.(message.ID)

	if tipSelector.tips.Size() == 1 {
		parent1MessageID = parent2MessageID
		return
	}

	parent1MessageID = tipSelector.tips.RandomEntry().(message.ID)
	for parent1MessageID == parent2MessageID && tipSelector.tips.Size() > 1 {
		parent1MessageID = tipSelector.tips.RandomEntry().(message.ID)
	}

	return
}

// TipCount the amount of current tips.
func (tipSelector *TipSelector) TipCount() int {
	return tipSelector.tips.Size()
}
