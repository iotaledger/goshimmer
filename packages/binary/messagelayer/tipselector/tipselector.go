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
func New() *TipSelector {
	return &TipSelector{
		tips: datastructure.NewRandomMap(),
		Events: Events{
			TipAdded:   events.NewEvent(messageIdEvent),
			TipRemoved: events.NewEvent(messageIdEvent),
		},
	}
}

// AddTip adds the given message as a tip.
func (tipSelector *TipSelector) AddTip(msg *message.Message) {
	messageId := msg.Id()
	if tipSelector.tips.Set(messageId, messageId) {
		tipSelector.Events.TipAdded.Trigger(messageId)
	}

	trunkMessageId := msg.TrunkId()
	if _, deleted := tipSelector.tips.Delete(trunkMessageId); deleted {
		tipSelector.Events.TipRemoved.Trigger(trunkMessageId)
	}

	branchMessageId := msg.BranchId()
	if _, deleted := tipSelector.tips.Delete(branchMessageId); deleted {
		tipSelector.Events.TipRemoved.Trigger(branchMessageId)
	}
}

// Tips returns two tips.
func (tipSelector *TipSelector) Tips() (trunkMessageId, branchMessageId message.Id) {
	tip := tipSelector.tips.RandomEntry()
	if tip == nil {
		trunkMessageId = message.EmptyId
		branchMessageId = message.EmptyId

		return
	}

	branchMessageId = tip.(message.Id)

	if tipSelector.tips.Size() == 1 {
		trunkMessageId = branchMessageId
		return
	}

	trunkMessageId = tipSelector.tips.RandomEntry().(message.Id)
	for trunkMessageId == branchMessageId && tipSelector.tips.Size() > 1 {
		trunkMessageId = tipSelector.tips.RandomEntry().(message.Id)
	}

	return
}

// TipCount the amount of current tips.
func (tipSelector *TipSelector) TipCount() int {
	return tipSelector.tips.Size()
}
