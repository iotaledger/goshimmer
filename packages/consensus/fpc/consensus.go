package fpc

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

type Consensus struct {
	tangle                   *tangle.Tangle
	PayloadOpinionProvider   OpinionVoterProvider
	TimestampOpinionProvider OpinionProvider

	waiting *opinionWait
}

func New(payloadOpinionVoterProvider OpinionVoterProvider, timestampOpinionProvider OpinionProvider) *Consensus {
	return &Consensus{
		PayloadOpinionProvider:   payloadOpinionVoterProvider,
		TimestampOpinionProvider: timestampOpinionProvider,
		waiting:                  &opinionWait{waitMap: make(map[tangle.MessageID]*waitStruct)},
	}
}

func (c *Consensus) Setup(tangle *tangle.Tangle) {
	c.tangle = tangle

	c.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(c.EvaluateMessage))

	c.PayloadOpinionProvider.Setup(c.tangle.OpinionManager.Events.PayloadOpinionFormed)
	c.TimestampOpinionProvider.Setup(c.tangle.OpinionManager.Events.TimestampOpinionFormed)

	c.tangle.OpinionManager.Events.PayloadOpinionFormed.Attach(events.NewClosure(c.onPayloadOpinionFormed))
	c.tangle.OpinionManager.Events.TimestampOpinionFormed.Attach(events.NewClosure(c.onTimestampOpinionFormed))
}

func (c *Consensus) EvaluateMessage(messageID tangle.MessageID) {
	c.PayloadOpinionProvider.Evaluate(messageID)
	c.TimestampOpinionProvider.Evaluate(messageID)
}

func (c *Consensus) PayloadLiked(messageID tangle.MessageID) (liked bool) {
	liked = true
	if !c.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		liked = c.PayloadOpinionProvider.Opinion(messageID)
	}) {
		if !c.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {}) {
			liked = false
		}
	}

	return
}

func (c *Consensus) Shutdown() {
	c.PayloadOpinionProvider.Shutdown()
	c.TimestampOpinionProvider.Shutdown()
}

func (c *Consensus) onPayloadOpinionFormed(ev *tangle.OpinionFormedEvent) {
	isTxConfirmed := false
	// set BranchLiked and BranchFinalized if this payload was a conflict
	c.tangle.Utils.ComputeIfTransaction(ev.MessageID, func(transactionID ledgerstate.TransactionID) {
		c.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			transactionMetadata.SetFinalized(true)
		})
		if c.tangle.LedgerState.TransactionConflicting(transactionID) {
			c.tangle.LedgerState.BranchDAG.SetBranchLiked(c.tangle.LedgerState.BranchID(transactionID), ev.Opinion)
			// TODO: move this to approval weight logic
			c.tangle.LedgerState.BranchDAG.SetBranchFinalized(c.tangle.LedgerState.BranchID(transactionID), true)
			isTxConfirmed = ev.Opinion
		}
	})

	if c.waiting.done(ev.MessageID, payloadOpinion) {
		c.setEligibility(ev.MessageID)
		// trigger TransactionOpinionFormed if the message contains a transaction
		if isTxConfirmed {
			c.tangle.OpinionManager.Events.TransactionConfirmed.Trigger(ev.MessageID)
		}
		c.tangle.OpinionManager.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (c *Consensus) onTimestampOpinionFormed(messageID tangle.MessageID) {
	c.setEligibility(messageID)
	if c.waiting.done(messageID, timestampOpinion) {
		c.tangle.OpinionManager.Events.MessageOpinionFormed.Trigger(messageID)
	}
}

func (c *Consensus) setEligibility(messageID tangle.MessageID) {
	c.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		eligible := c.parentsEligibility(messageID) &&
			messageMetadata.TimestampOpinion().Value == opinion.Like &&
			messageMetadata.TimestampOpinion().LoK > tangle.One

		messageMetadata.SetEligible(eligible)
	})
}

// parentsEligibility checks if the parents are eligible.
func (c *Consensus) parentsEligibility(messageID tangle.MessageID) (eligible bool) {
	c.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent tangle.Parent) {
			if eligible = eligible && c.tangle.OpinionManager.MessageEligible(parent.ID); !eligible {
				return
			}
		})
	})
	return
}

// OpinionProvider is the interface to describe the functionalities of an opinion provider.
type OpinionProvider interface {
	// Evaluate evaluates the opinion of the given messageID (payload / timestamp).
	Evaluate(tangle.MessageID)
	// Opinion returns the opinion of the given messageID (payload / timestamp).
	Opinion(tangle.MessageID) bool
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
	// TransactionOpinionEssence returns the opinion essence of a given transactionID.
	TransactionOpinionEssence(ledgerstate.TransactionID) tangle.OpinionEssence
}

type callerType uint8

const (
	payloadOpinion callerType = iota
	timestampOpinion
)

type waitStruct struct {
	payloadCaller   bool
	timestampCaller bool
}

func (w *waitStruct) update(caller callerType) {
	switch caller {
	case payloadOpinion:
		w.payloadCaller = true
	default:
		w.timestampCaller = true
	}
}

func (w *waitStruct) ready() (ready bool) {
	return w.payloadCaller && w.timestampCaller
}

type opinionWait struct {
	waitMap map[tangle.MessageID]*waitStruct
	sync.Mutex
}

func (o *opinionWait) done(messageID tangle.MessageID, caller callerType) (done bool) {
	o.Lock()
	defer o.Unlock()
	if _, exist := o.waitMap[messageID]; !exist {
		o.waitMap[messageID] = &waitStruct{}
	}
	o.waitMap[messageID].update(caller)
	if o.waitMap[messageID].ready() {
		delete(o.waitMap, messageID)
		done = true
	}
	return
}
