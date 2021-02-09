package tangle

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
)

// OpinionProvider is the interface to describe the functionalities of an opinion provider:
// Evaluate evaluates the opinion of the given messageID (payload / timestamp);
// Opinion returns the opinion of the given messageID (payload / timestamp);
// SetupEvent allows to wire the communication between the opinionProvider and the opinionFormer.
type OpinionProvider interface {
	Evaluate(MessageID)
	Opinion(MessageID) bool
	SetupEvent(*events.Event)
}

// Events defines all the events related to the opinion manager.
type OpinionFormerEvents struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event

	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event
}

// OpinionFormedEvent holds data about a Payload/MessageOpinionFormed event.
type OpinionFormedEvent struct {
	// The messageID of the message containing the payload.
	MessageID MessageID
	// The opinion of the payload.
	Opinion bool
}

func payloadOpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*OpinionFormedEvent))(params[0].(*OpinionFormedEvent))
}

type OpinionFormer struct {
	Events OpinionFormerEvents

	tangle  *Tangle
	waiting *opinionWait

	opinionPayloadProvider   OpinionProvider
	opinionTimestampProvider OpinionProvider
}

func NewOpinionFormer(tangle *Tangle, opinionPayloadManager, opinionTimestampManager OpinionProvider) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:                   tangle,
		waiting:                  &opinionWait{waitMap: make(map[MessageID]types.Empty)},
		opinionPayloadProvider:   opinionPayloadManager,
		opinionTimestampProvider: opinionTimestampManager,
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
		},
	}

	return
}

func (o *OpinionFormer) Setup() {
	o.opinionPayloadProvider.SetupEvent(o.Events.PayloadOpinionFormed)
	o.opinionTimestampProvider.SetupEvent(o.Events.TimestampOpinionFormed)
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.opinionPayloadProvider.Evaluate))
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.opinionTimestampProvider.Evaluate))

	o.Events.PayloadOpinionFormed.Attach(events.NewClosure(o.onPayloadOpinionFormed))
	o.Events.TimestampOpinionFormed.Attach(events.NewClosure(o.onTimestampOpinionFormed))
}

func (o *OpinionFormer) PayloadLiked(messageID MessageID) (liked bool) {
	return o.opinionPayloadProvider.Opinion(messageID)
}

// isMessageEligible returns whether the given messageID is marked as aligible.
func (o *OpinionFormer) MessageEligible(messageID MessageID) (eligible bool) {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = messageMetadata.IsEligible()
	})

	return
}

func (o *OpinionFormer) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
	// set BranchLiked and BranchFinalized if this payload was a conflict
	o.tangle.Storage.Message(ev.MessageID).Consume(func(message *Message) {
		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			if o.tangle.LedgerState.TransactionIsConflicting(transactionID) {
				o.tangle.LedgerState.branchDAG.SetBranchLiked(o.tangle.LedgerState.BranchID(transactionID), ev.Opinion)
				// TODO: move this to approval weight logic
				o.tangle.LedgerState.branchDAG.SetBranchFinalized(o.tangle.LedgerState.BranchID(transactionID), true)
			}
		}
	})

	if o.waiting.done(ev.MessageID) {
		o.setEligibility(ev.MessageID)
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onTimestampOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) {
		o.setEligibility(ev.MessageID)
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) setEligibility(messageID MessageID) {
	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible := o.parentsEligibility(messageID) &&
			messageMetadata.TimestampOpinion().Value == opinion.Like &&
			messageMetadata.TimestampOpinion().LoK > One

		messageMetadata.SetEligible(eligible)
	})
	return
}

// parentsEligibility checks if the parents are eligible.
func (o *OpinionFormer) parentsEligibility(messageID MessageID) (eligible bool) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent Parent) {
			eligible = eligible && o.MessageEligible(parent.ID)
		})
	})
	return
}

type opinionWait struct {
	waitMap map[MessageID]types.Empty
	sync.Mutex
}

func (o *opinionWait) done(messageID MessageID) (done bool) {
	o.Lock()
	defer o.Unlock()
	if _, exist := o.waitMap[messageID]; !exist {
		o.waitMap[messageID] = types.Void
		return
	}
	delete(o.waitMap, messageID)
	done = true
	return
}
