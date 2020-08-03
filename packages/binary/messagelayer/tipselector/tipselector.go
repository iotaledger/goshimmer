package tipselector

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/datastructure"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// TipSelector manages a map of tips and emits events for their removal and addition.
type TipSelector struct {
	tips   *datastructure.RandomMap
	Events Events
}

// New creates a new tip-selector.
func New(tips ...message.ID) *TipSelector {
	tipSelector := &TipSelector{
		tips: datastructure.NewRandomMap(),
		Events: Events{
			TipAdded:   events.NewEvent(messageIDEvent),
			TipRemoved: events.NewEvent(messageIDEvent),
		},
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

	trunkMessageID := msg.TrunkID()
	if _, deleted := tipSelector.tips.Delete(trunkMessageID); deleted {
		tipSelector.Events.TipRemoved.Trigger(trunkMessageID)
	}

	branchMessageID := msg.BranchID()
	if _, deleted := tipSelector.tips.Delete(branchMessageID); deleted {
		tipSelector.Events.TipRemoved.Trigger(branchMessageID)
	}
}

// Tips returns two tips.
func (tipSelector *TipSelector) Tips() (trunkMessageID, branchMessageID message.ID) {
	tip := tipSelector.tips.RandomEntry()
	if tip == nil {
		trunkMessageID = message.EmptyID
		branchMessageID = message.EmptyID

		return
	}

	branchMessageID = tip.(message.ID)

	if tipSelector.tips.Size() == 1 {
		trunkMessageID = branchMessageID
		return
	}

	trunkMessageID = tipSelector.tips.RandomEntry().(message.ID)
	for trunkMessageID == branchMessageID && tipSelector.tips.Size() > 1 {
		trunkMessageID = tipSelector.tips.RandomEntry().(message.ID)
	}

	return
}

// TipCount the amount of current tips.
func (tipSelector *TipSelector) TipCount() int {
	return tipSelector.tips.Size()
}
