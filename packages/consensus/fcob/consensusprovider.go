package fcob

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
)

type ConsensusProvider struct {
	Events                 *Events
	PayloadOpinionProvider *FCoB

	tangle  *tangle.Tangle
	waiting *opinionWait
}

func NewConsensusProvider() *ConsensusProvider {
	return &ConsensusProvider{
		Events: &Events{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(tangle.MessageIDEventHandler),
		},
		waiting: &opinionWait{waitMap: make(map[tangle.MessageID]*waitStruct)},
	}
}

func (f *ConsensusProvider) Init(tangle *tangle.Tangle) {
	f.tangle = tangle
	f.PayloadOpinionProvider = NewFCoB(tangle)
}

func (f *ConsensusProvider) Setup() {
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.PayloadOpinionProvider.Evaluate))
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.EvaluateTimestamp))

	f.PayloadOpinionProvider.Setup(f.Events.PayloadOpinionFormed)

	f.Events.PayloadOpinionFormed.Attach(events.NewClosure(f.onPayloadOpinionFormed))
}

func (f *ConsensusProvider) EvaluateTimestamp(messageID tangle.MessageID) {
	f.PayloadOpinionProvider.StoreTimestampOpinion(&TimestampOpinion{
		MessageID: messageID,
		Value:     voter.Like,
		LoK:       Two,
	})

	f.setEligibility(messageID)

	if f.waiting.done(messageID, timestampOpinion) {
		f.tangle.OpinionManager.Events.MessageOpinionFormed.Trigger(messageID)
	}
}

func (f *ConsensusProvider) PayloadLiked(messageID tangle.MessageID) (liked bool) {
	return f.PayloadOpinionProvider.Opinion(messageID)
}

func (f *ConsensusProvider) Shutdown() {
	f.PayloadOpinionProvider.Shutdown()
}

func (f *ConsensusProvider) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
	isTxConfirmed := false
	// set BranchLiked and BranchFinalized if this payload was a conflict
	f.tangle.Utils.ComputeIfTransaction(ev.MessageID, func(transactionID ledgerstate.TransactionID) {
		isTxConfirmed = ev.Opinion
		f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			transactionMetadata.SetFinalized(true)
		})
		if f.tangle.LedgerState.TransactionConflicting(transactionID) {
			f.tangle.LedgerState.BranchDAG.SetBranchLiked(f.tangle.LedgerState.BranchID(transactionID), ev.Opinion)
			// TODO: move this to approval weight logic
			f.tangle.LedgerState.BranchDAG.SetBranchFinalized(f.tangle.LedgerState.BranchID(transactionID), true)
			isTxConfirmed = ev.Opinion
		}
	})

	if f.waiting.done(ev.MessageID, payloadOpinion) {
		f.setEligibility(ev.MessageID)
		// trigger TransactionOpinionFormed if the message contains a transaction
		if isTxConfirmed {
			f.tangle.OpinionManager.Events.TransactionConfirmed.Trigger(ev.MessageID)
		}
		f.tangle.OpinionManager.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (f *ConsensusProvider) setEligibility(messageID tangle.MessageID) {
	f.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		timestampOpinion := f.PayloadOpinionProvider.TimestampOpinion(messageID)

		messageMetadata.SetEligible(
			timestampOpinion != nil && timestampOpinion.Value == opinion.Like && timestampOpinion.LoK > One && f.parentsEligibility(messageID),
		)
	})
}

// parentsEligibility checks if the parents are eligible.
func (f *ConsensusProvider) parentsEligibility(messageID tangle.MessageID) (eligible bool) {
	f.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent tangle.Parent) {
			if eligible = eligible && f.tangle.OpinionManager.MessageEligible(parent.ID); !eligible {
				return
			}
		})
	})
	return
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
