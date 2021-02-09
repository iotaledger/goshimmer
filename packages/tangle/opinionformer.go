package tangle

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
)

// Opinioner is the interface to describe the functionalities of an opinioner.
type Opinioner interface {
	Evaluate(messageID MessageID)
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

	opinionPayloadManager   Opinioner
	opinionTimestampManager Opinioner
}

func NewOpinionFormer(tangle *Tangle, opinionPayloadManager, opinionTimestampManager Opinioner) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:                  tangle,
		waiting:                 &opinionWait{waitMap: make(map[MessageID]types.Empty)},
		opinionPayloadManager:   opinionPayloadManager,
		opinionTimestampManager: opinionTimestampManager,
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
		},
	}

	return
}

func (o *OpinionFormer) Setup() {
	o.opinionPayloadManager.SetupEvent(o.Events.PayloadOpinionFormed)
	o.opinionTimestampManager.SetupEvent(o.Events.TimestampOpinionFormed)
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.opinionPayloadManager.Evaluate))
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.opinionTimestampManager.Evaluate))

	o.Events.PayloadOpinionFormed.Attach(events.NewClosure(o.onPayloadOpinionFormed))
	o.Events.TimestampOpinionFormed.Attach(events.NewClosure(o.onTimestampOpinionFormed))
}

func (o *OpinionFormer) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
	// TODO: we should propagate according to monotonicity
	// branch of the message
	// if this guys was a conflict {
	// o.tangle.LedgerState.branchDAG.SetBranchLiked()
	// set branch Finalized
	//}
	// set the transaction finalized
	if o.waiting.done(ev.MessageID) && o.eligible(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onTimestampOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) && o.eligible(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) eligible(messageID MessageID) (eligible bool) {
	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = o.parentsEligibility(messageID) &&
			messageMetadata.TimestampOpinion().Value == opinion.Like &&
			messageMetadata.TimestampOpinion().LoK > One

		messageMetadata.SetEligible(eligible)
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

// parentsEligibility checks if the parents are eligible.
func (o *OpinionFormer) parentsEligibility(messageID MessageID) (eligible bool) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent Parent) {
			eligible = eligible && o.isMessageEligible(parent.ID)
		})
	})
	return
}

// isMessageEligible returns whether the given messageID is marked as aligible.
func (o *OpinionFormer) isMessageEligible(messageID MessageID) (eligible bool) {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible = messageMetadata.IsEligible()
	})

	return
}
