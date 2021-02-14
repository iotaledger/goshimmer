package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/datastructure/randommap"
	"github.com/iotaledger/hive.go/events"
)

// region TipType //////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// StrongTip is the TipType that represents strong tips, i.e., eligible messages that are in a monotonically liked branch.
	StrongTip TipType = iota

	// WeakTip is the TipType that represents weak tips, i.e., eligible messages that are in a not monotonically liked branch.
	WeakTip
)

// TipType is the type (weak/strong) of the tip.
type TipType uint8

// String returns a human readable version of the TipType.
func (t TipType) String() string {
	switch t {
	case StrongTip:
		return "TipType(StrongTip)"
	case WeakTip:
		return "TipType(WeakTip)"
	default:
		return fmt.Sprintf("TipType(%X)", uint8(t))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

// TipManager manages a map of tips and emits events for their removal and addition.
type TipManager struct {
	tangle     *Tangle
	strongTips *randommap.RandomMap
	weakTips   *randommap.RandomMap
	Events     *TipManagerEvents
}

// NewTipManager creates a new tip-selector.
func NewTipManager(tangle *Tangle, tips ...MessageID) *TipManager {
	tipSelector := &TipManager{
		tangle:     tangle,
		strongTips: randommap.New(),
		weakTips:   randommap.New(),
		Events: &TipManagerEvents{
			TipAdded:   events.NewEvent(tipEventHandler),
			TipRemoved: events.NewEvent(tipEventHandler),
		},
	}

	if tips != nil {
		tipSelector.Set(tips...)
	}

	return tipSelector
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TipManager) Setup() {
	t.tangle.OpinionFormer.Events.MessageOpinionFormed.Attach(events.NewClosure(func(messageID MessageID) {
		t.tangle.Storage.Message(messageID).Consume(t.AddTip)
	}))
}

// Set adds the given messageIDs as tips.
func (t *TipManager) Set(tips ...MessageID) {
	for _, messageID := range tips {
		t.strongTips.Set(messageID, messageID)
	}
}

// AddTip first checks whether the message is eligible and its payload liked. If yes, then the given message is added as
// a strong or weak tip depending on its branch status. Parents of a message that are currently tip lose the tip status
// and are removed.
func (t *TipManager) AddTip(message *Message) {
	messageID := message.ID()
	cachedMessageMetadata := t.tangle.Storage.MessageMetadata(messageID)
	messageMetadata := cachedMessageMetadata.Unwrap()
	defer cachedMessageMetadata.Release()

	if messageMetadata == nil {
		panic(fmt.Errorf("failed to load MessageMetadata with %s", messageID))
	}

	if !messageMetadata.IsEligible() {
		return
	}

	if !t.tangle.OpinionFormer.PayloadLiked(messageID) {
		return
	}

	// TODO: possible logical race condition if a child message gets added before its parents.
	//  To be sure we probably need to check "It is not directly referenced by any strong message via strong/weak parent"
	//  before adding a message as a tip. For now we're using only 1 worker after the scheduler and it shouldn't be a problem.

	// if branch is monotonically liked: strong message
	// if branch is not monotonically liked: weak message
	t.tangle.LedgerState.branchDAG.Branch(messageMetadata.BranchID()).Consume(func(branch ledgerstate.Branch) {
		if branch.MonotonicallyLiked() {
			if t.strongTips.Set(messageID, messageID) {
				t.Events.TipAdded.Trigger(&TipEvent{
					MessageID: messageID,
					TipType:   StrongTip,
				})
			}

			// a strong tip loses its tip status if it is referenced by a strong message via strong parent
			message.ForEachStrongParent(func(parent MessageID) {
				if _, deleted := t.strongTips.Delete(parent); deleted {
					t.Events.TipRemoved.Trigger(&TipEvent{
						MessageID: parent,
						TipType:   StrongTip,
					})
				}
			})
			// a weak tip loses its tip status if it is referenced by a strong message via weak parent
			message.ForEachWeakParent(func(parent MessageID) {
				if _, deleted := t.weakTips.Delete(parent); deleted {
					t.Events.TipRemoved.Trigger(&TipEvent{
						MessageID: parent,
						TipType:   WeakTip,
					})
				}
			})
		} else {
			if t.weakTips.Set(messageID, messageID) {
				t.Events.TipAdded.Trigger(&TipEvent{
					MessageID: messageID,
					TipType:   WeakTip,
				})
			}
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

	tips := t.strongTips.RandomUniqueEntries(count)
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

// StrongTipCount the amount of strong tips.
func (t *TipManager) StrongTipCount() int {
	return t.strongTips.Size()
}

// WeakTipCount the amount of weak tips.
func (t *TipManager) WeakTipCount() int {
	return t.weakTips.Size()
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

// region TipEvent /////////////////////////////////////////////////////////////////////////////////////////////////////

// TipEvent holds the information provided by the TipEvent event that gets triggered when a message gets added or
// removed as tip.
type TipEvent struct {
	// MessageID of the added/removed tip.
	MessageID MessageID

	// TipType is the type of the added/removed tip.
	TipType TipType
}

func tipEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(event *TipEvent))(params[0].(*TipEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
