package tangle

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
)

// OpinionProvider is the interface to describe the functionalities of an opinion provider.
type OpinionProvider interface {
	// Evaluate evaluates the opinion of the given messageID (payload / timestamp).
	Evaluate(MessageID)
	// Opinion returns the opinion of the given messageID (payload / timestamp).
	Opinion(MessageID) bool
	// Setup allows to wire the communication between the opinionProvider and the opinionFormer.
	Setup(*events.Event)
	// Shutdown shuts down the OpinionProvider and persists its state.
	Shutdown()
}

// OpinionVoterProvider is the interface to describe the functionalities of an opinion and voter provider.
type OpinionVoterProvider interface {
	// OpinionProvider is the interface to describe the functionalities of an opinion provider.
	OpinionProvider
	// Vote trigger a voting request.
	Vote() *events.Event
	// VoteError notify an error coming from the result of voting.
	VoteError() *events.Event
	// ProcessVote allows an external voter to hand in the results of the voting process.
	ProcessVote(*vote.OpinionEvent)
}

// OpinionFormerEvents defines all the events related to the opinion manager.
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

// OpinionFormer is the component in charge of forming opinion about timestamps and payloads.
type OpinionFormer struct {
	Events OpinionFormerEvents

	tangle  *Tangle
	waiting *opinionWait

	payloadOpinionProvider   OpinionVoterProvider
	TimestampOpinionProvider OpinionProvider
}

// NewOpinionFormer returns a new OpinionFormer.
func NewOpinionFormer(tangle *Tangle, payloadOpinionVoterProvider OpinionVoterProvider, timestampOpinionProvider OpinionProvider) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:                   tangle,
		waiting:                  &opinionWait{waitMap: make(map[MessageID]types.Empty)},
		payloadOpinionProvider:   payloadOpinionVoterProvider,
		TimestampOpinionProvider: timestampOpinionProvider,
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
		},
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (o *OpinionFormer) Setup() {
	o.payloadOpinionProvider.Setup(o.Events.PayloadOpinionFormed)
	o.TimestampOpinionProvider.Setup(o.Events.TimestampOpinionFormed)
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.payloadOpinionProvider.Evaluate))
	o.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(o.TimestampOpinionProvider.Evaluate))

	o.Events.PayloadOpinionFormed.Attach(events.NewClosure(o.onPayloadOpinionFormed))
	o.Events.TimestampOpinionFormed.Attach(events.NewClosure(o.onTimestampOpinionFormed))
}

// Shutdown shuts down the component and persists its state.
func (o *OpinionFormer) Shutdown() {
	o.payloadOpinionProvider.Shutdown()
	o.TimestampOpinionProvider.Shutdown()
}

// PayloadLiked returns the opinion of the given MessageID.
func (o *OpinionFormer) PayloadLiked(messageID MessageID) (liked bool) {
	liked = true
	if !o.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		liked = o.payloadOpinionProvider.Opinion(messageID)
	}) {
		if !o.tangle.Storage.Message(messageID).Consume(func(message *Message) {}) {
			liked = false
		}
	}

	return
}

// MessageEligible returns whether the given messageID is marked as aligible.
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
	o.tangle.Utils.ComputeIfTransaction(ev.MessageID, func(transactionID ledgerstate.TransactionID) {
		if o.tangle.LedgerState.TransactionConflicting(transactionID) {
			o.tangle.LedgerState.branchDAG.SetBranchLiked(o.tangle.LedgerState.BranchID(transactionID), ev.Opinion)
			// TODO: move this to approval weight logic
			o.tangle.LedgerState.branchDAG.SetBranchFinalized(o.tangle.LedgerState.BranchID(transactionID), true)
		}
	})

	if o.waiting.done(ev.MessageID) {
		o.setEligibility(ev.MessageID)
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onTimestampOpinionFormed(messageID MessageID) {
	if o.waiting.done(messageID) {
		o.setEligibility(messageID)
		o.Events.MessageOpinionFormed.Trigger(messageID)
	}
}

func (o *OpinionFormer) setEligibility(messageID MessageID) {
	o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		eligible := o.parentsEligibility(messageID) &&
			messageMetadata.TimestampOpinion().Value == opinion.Like &&
			messageMetadata.TimestampOpinion().LoK > One

		messageMetadata.SetEligible(eligible)
	})
}

// parentsEligibility checks if the parents are eligible.
func (o *OpinionFormer) parentsEligibility(messageID MessageID) (eligible bool) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent Parent) {
			if eligible = eligible && o.MessageEligible(parent.ID); !eligible {
				return
			}
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
