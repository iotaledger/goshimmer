package fcob

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/timedqueue"
)

var (
	// LikedThreshold is the first time threshold of FCoB.
	LikedThreshold = 5 * time.Second

	// LocallyFinalizedThreshold is the second time threshold of FCoB.
	LocallyFinalizedThreshold = 10 * time.Second
)

// region ConsensusMechanism ///////////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanism represents the FCoB consensus that can be used as a ConsensusMechanism in the Tangle.
type ConsensusMechanism struct {
	Events *ConsensusMechanismEvents

	tangle                   *tangle.Tangle
	storage                  *Storage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
	waiting                  *opinionWait
}

// NewConsensusMechanism is the constructor for the FCoB consensus mechanism.
func NewConsensusMechanism() *ConsensusMechanism {
	return &ConsensusMechanism{
		Events: &ConsensusMechanismEvents{
			Error: events.NewEvent(events.ErrorCaller),
			Vote:  events.NewEvent(voteEventHandler),
		},
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		waiting:                  &opinionWait{waitMap: make(map[tangle.MessageID]*waitStruct)},
	}
}

// Init initializes the ConsensusMechanism by making the Tangle object available that is using it.
func (f *ConsensusMechanism) Init(tangle *tangle.Tangle) {
	f.tangle = tangle
	f.storage = NewStorage(tangle.Options.Store)
}

// Setup sets up the behavior of the ConsensusMechanism by making it attach to the relevant events in the Tangle.
func (f *ConsensusMechanism) Setup() {
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.Evaluate))
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.EvaluateTimestamp))
}

// TransactionLiked returns a boolean value indicating whether the given Transaction is liked.
func (f *ConsensusMechanism) TransactionLiked(transactionID ledgerstate.TransactionID) (liked bool) {
	f.storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
		liked = opinion.OpinionEssence.liked
	})

	return
}

// Shutdown shuts down the ConsensusMechanism and persists its state.
func (f *ConsensusMechanism) Shutdown() {
	f.likedThresholdExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.locallyFinalizedExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.storage.Shutdown()
}

// Evaluate evaluates the opinion of the given messageID.
func (f *ConsensusMechanism) Evaluate(messageID tangle.MessageID) {
	if f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		f.onTransactionBooked(transactionID, messageID)
	}) {
		return
	}

	// likes by default all non-value-transaction messages
	f.onPayloadOpinionFormed(messageID, true)
}

// EvaluateTimestamp evaluates the honesty of the timestamp of the given Message.
func (f *ConsensusMechanism) EvaluateTimestamp(messageID tangle.MessageID) {
	f.storage.StoreTimestampOpinion(&TimestampOpinion{
		MessageID: messageID,
		Value:     voter.Like,
		LoK:       Two,
	})

	f.setEligibility(messageID)

	if f.waiting.done(messageID, timestampOpinion) {
		f.tangle.ConsensusManager.Events.MessageOpinionFormed.Trigger(messageID)
	}
}

// ProcessVote allows an external voter to hand in the results of the voting process.
func (f *ConsensusMechanism) ProcessVote(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			f.Events.Error.Trigger(err)
			return
		}

		f.storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == voter.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
			for _, messageID := range messageIDs {
				f.onPayloadOpinionFormed(messageID, opinion.liked)
			}
		})
	}
}

// TransactionOpinionEssence returns the opinion essence of a given transactionID.
func (f *ConsensusMechanism) TransactionOpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	opinion = f.storage.OpinionEssence(transactionID)

	return
}

// OpinionsEssence returns a list of OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given conflictSet.
func (f *ConsensusMechanism) OpinionsEssence(conflictSet ledgerstate.TransactionIDs) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, 0)
	for conflictID := range conflictSet {
		opinions = append(opinions, f.storage.OpinionEssence(conflictID))
	}
	return
}

func (f *ConsensusMechanism) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	isReattachment := false
	f.storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
		// if the opinion has been already set by the opinion provider, re-use it
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			f.onPayloadOpinionFormed(messageID, opinion.liked)
		}
		// otherwise the PayloadOpinionFormed will be triggered by iterating over the Attachments
		// either after FCOB or as a result of an FPC voting.
		isReattachment = true
	})
	if isReattachment {
		return
	}

	newOpinion := &Opinion{
		transactionID: transactionID,
	}
	var timestamp time.Time
	f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	// filters both rejected and invalid branch
	branchInclusionState := f.tangle.LedgerState.BranchInclusionState(f.tangle.LedgerState.BranchID(transactionID))
	if branchInclusionState == ledgerstate.Rejected {
		newOpinion.OpinionEssence = OpinionEssence{
			timestamp:        timestamp,
			liked:            false,
			levelOfKnowledge: Two,
		}
		f.storage.opinionStorage.Store(newOpinion).Release()
		f.onPayloadOpinionFormed(messageID, newOpinion.liked)
		return
	}

	if f.tangle.LedgerState.TransactionConflicting(transactionID) {
		newOpinion.OpinionEssence = deriveOpinion(timestamp, f.OpinionsEssence(f.tangle.LedgerState.ConflictSet(transactionID)))

		cachedOpinion := f.storage.opinionStorage.Store(newOpinion)
		defer cachedOpinion.Release()

		if newOpinion.LevelOfKnowledge() == One {
			// trigger voting for this transactionID
			liked := voter.Dislike
			if newOpinion.liked {
				liked = voter.Like
			}
			f.Events.Vote.Trigger(transactionID.Base58(), liked)
		}

		if newOpinion.LevelOfKnowledge() > One {
			f.onPayloadOpinionFormed(messageID, newOpinion.liked)
		}

		return
	}

	newOpinion.OpinionEssence = OpinionEssence{
		timestamp:        timestamp,
		levelOfKnowledge: Pending,
	}
	cachedOpinion := f.storage.opinionStorage.Store(newOpinion)
	defer cachedOpinion.Release()

	// Wait LikedThreshold
	f.likedThresholdExecutor.ExecuteAt(func() {
		f.storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLevelOfKnowledge(One)
			if f.tangle.LedgerState.TransactionConflicting(transactionID) {
				opinion.SetLiked(false)
				// trigger voting for this transactionID
				f.Events.Vote.Trigger(transactionID.Base58(), voter.Dislike)
				return
			}
			opinion.SetLiked(true)
		})

		// Wait LocallyFinalizedThreshold
		f.locallyFinalizedExecutor.ExecuteAt(func() {
			f.storage.Opinion(transactionID).Consume(func(opinion *Opinion) {
				opinion.SetLiked(true)
				if f.tangle.LedgerState.TransactionConflicting(transactionID) {
					// trigger voting for this transactionID
					f.Events.Vote.Trigger(transactionID.Base58(), voter.Like)
					return
				}
				opinion.SetLevelOfKnowledge(Two)
				// trigger OpinionPayloadFormed
				messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
				for _, messageID := range messageIDs {
					f.onPayloadOpinionFormed(messageID, opinion.liked)
				}
			})
		}, timestamp.Add(LocallyFinalizedThreshold))
	}, timestamp.Add(LikedThreshold))
}

func (f *ConsensusMechanism) onPayloadOpinionFormed(messageID tangle.MessageID, liked bool) {
	isTxConfirmed := false
	// set BranchLiked and BranchFinalized if this payload was a conflict
	f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		isTxConfirmed = liked
		f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			transactionMetadata.SetFinalized(true)
		})
		if f.tangle.LedgerState.TransactionConflicting(transactionID) {
			_, _ = f.tangle.LedgerState.SetBranchLiked(f.tangle.LedgerState.BranchID(transactionID), liked)
			// TODO: move this to approval weight logic
			_, _ = f.tangle.LedgerState.SetBranchFinalized(f.tangle.LedgerState.BranchID(transactionID), true)
			isTxConfirmed = liked
		}
	})

	if f.waiting.done(messageID, payloadOpinion) {
		f.setEligibility(messageID)
		// trigger TransactionOpinionFormed if the message contains a transaction
		if isTxConfirmed {
			f.tangle.ConsensusManager.Events.TransactionConfirmed.Trigger(messageID)
		}
		f.tangle.ConsensusManager.Events.MessageOpinionFormed.Trigger(messageID)
	}
}

func (f *ConsensusMechanism) setEligibility(messageID tangle.MessageID) {
	f.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		f.storage.TimestampOpinion(messageID).Consume(func(timestampOpinion *TimestampOpinion) {
			messageMetadata.SetEligible(
				timestampOpinion != nil && timestampOpinion.Value == opinion.Like && timestampOpinion.LoK > One && f.parentsEligibility(messageID),
			)
		})
	})
}

// parentsEligibility checks if the parents are eligible.
func (f *ConsensusMechanism) parentsEligibility(messageID tangle.MessageID) (eligible bool) {
	f.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		eligible = true
		// check if all the parents are eligible
		message.ForEachParent(func(parent tangle.Parent) {
			if eligible = eligible && f.tangle.ConsensusManager.MessageEligible(parent.ID); !eligible {
				return
			}
		})
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConsensusMechanismEvents /////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanismEvents represents events triggered by the ConsensusMechanism.
type ConsensusMechanismEvents struct {
	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote.
	Vote *events.Event
}

func voteEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn voter.Opinion))(params[0].(string), params[1].(voter.Opinion))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region utility functions ////////////////////////////////////////////////////////////////////////////////////////////

// deriveOpinion returns the initial opinion based on the given targetTime and conflictSet.
func deriveOpinion(targetTime time.Time, conflictSet ConflictSet) (opinion OpinionEssence) {
	if conflictSet.hasDecidedLike() {
		opinion = OpinionEssence{
			timestamp:        targetTime,
			liked:            false,
			levelOfKnowledge: Two,
		}
		return
	}

	anchor := conflictSet.anchor()
	if anchor == nil {
		opinion = OpinionEssence{
			timestamp:        targetTime,
			levelOfKnowledge: Pending,
		}
		return
	}

	opinion = OpinionEssence{
		timestamp:        targetTime,
		liked:            false,
		levelOfKnowledge: One,
	}
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
