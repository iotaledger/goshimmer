package consensus

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
)

// FCoBEvents defines all the events related to FCoB.
type FCoBEvents struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event

	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote.
	Vote *events.Event
}

func voteEvent(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn voter.Opinion))(params[0].(string), params[1].(voter.Opinion))
}

func messageIDEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(tangle.MessageID))(params[0].(tangle.MessageID))
}

// OpinionFormedEvent holds data about a Payload/MessageOpinionFormed event.
type OpinionFormedEvent struct {
	// The messageID of the message containing the payload.
	MessageID tangle.MessageID
	// The opinion of the payload.
	Opinion bool
}

func payloadOpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*OpinionFormedEvent))(params[0].(*OpinionFormedEvent))
}

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second

	onMessageBooked = events.NewClosure(func(messageID tangle.MessageID) {})
)

type FCoB struct {
	Events *FCoBEvents

	tangle *tangle.Tangle

	opinionStorage           *objectstorage.ObjectStorage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
}

func NewFCoB(store kvstore.KVStore, tangle *tangle.Tangle) (fcob *FCoB) {
	fcob = &FCoB{
		tangle:                   tangle,
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		Events: &FCoBEvents{
			Error: events.NewEvent(events.ErrorCaller),
			Vote:  events.NewEvent(voteEvent),
		},
	}

	return
}

func (o *FCoB) Setup(payloadEvent, timestampEvent *events.Event) {
	o.Events.PayloadOpinionFormed = payloadEvent
	o.Events.TimestampOpinionFormed = timestampEvent
}

func (f *FCoB) OnMessageBooked(messageID tangle.MessageID) {
	f.tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		// TODO: add timestamp quality evaluation.
		// For now we set all timestamps as good.
		f.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			messageMetadata.SetTimestampOpinion(tangle.TimestampOpinion{
				Value: voter.Like,
				LoK:   Two,
			})
			f.Events.TimestampOpinionFormed.Trigger(messageID)
		})

		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			f.onTransactionBooked(transactionID, messageID)
			return
		}
		// likes by default all non-value-transaction messages
		f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, true})
	})
}

func (f *FCoB) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		}
		return
	})

	opinion := &Opinion{
		transactionID: transactionID,
	}
	var timestamp time.Time
	f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	if f.isConflicting(transactionID) {
		opinion.OpinionEssence = deriveOpinion(timestamp, f.OpinionsEssence(f.conflictSet(transactionID)))

		cachedOpinion := f.opinionStorage.Store(opinion)
		defer cachedOpinion.Release()

		if opinion.LevelOfKnowledge() == One {
			//trigger voting for this transactionID
			f.Events.Vote.Trigger(transactionID.String(), opinion.liked) // TODO: fix opinion type
		}

		return
	}

	opinion.OpinionEssence = OpinionEssence{
		timestamp:        timestamp,
		levelOfKnowledge: Pending,
	}
	cachedOpinion := f.opinionStorage.Store(opinion)
	defer cachedOpinion.Release()

	// Wait LikedThreshold
	f.likedThresholdExecutor.ExecuteAt(func() {
		f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLevelOfKnowledge(One)
			if f.isConflicting(transactionID) {
				opinion.SetLiked(false)
				//trigger voting for this transactionID
				f.Events.Vote.Trigger(transactionID.String(), voter.Dislike)
				return
			}
			opinion.SetLiked(true)
		})

		// Wait LocallyFinalizedThreshold
		f.locallyFinalizedExecutor.ExecuteAt(func() {
			f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
				opinion.SetLiked(true)
				if f.isConflicting(transactionID) {
					//trigger voting for this transactionID
					f.Events.Vote.Trigger(transactionID.String(), voter.Like)
					return
				}
				opinion.SetLevelOfKnowledge(Two)
				// trigger OpinionPayloadFormed
				f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
			})
		}, timestamp.Add(LocallyFinalizedThreshold))
	}, timestamp.Add(LikedThreshold))
}

// ProcessVoteResult allows an external voter to hand in the results of the voting process.
func (o *FCoB) ProcessVoteResult(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			o.Events.Error.Trigger(err)

			return
		}

		// TODO: Check monotonicity and consistency among the conflict set.

		o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == voter.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			// o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		})
	}
}

func (o *FCoB) OpinionsEssence(conflictSet []ledgerstate.TransactionID) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, len(conflictSet))
	for i, conflictID := range conflictSet {
		opinions[i] = o.OpinionEssence(conflictID)
	}
	return
}

func (o *FCoB) isConflicting(transactionID ledgerstate.TransactionID) bool {
	cachedTransactionMetadata := o.tangle.LedgerState.TransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	return transactionMetadata.BranchID() == ledgerstate.NewBranchID(transactionID)
}

func (o *FCoB) conflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
	return o.tangle.LedgerState.ConflictSet(transactionID)
}

func (o *FCoB) OpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	(&CachedOpinion{CachedObject: o.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion.OpinionEssence
	})

	return
}

func (o *FCoB) CachedOpinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
	cachedOpinion = &CachedOpinion{CachedObject: o.opinionStorage.Load(transactionID.Bytes())}
	return
}

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
