package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote.
	Vote *events.Event
}

func voteEvent(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn voter.Opinion))(params[0].(string), params[1].(voter.Opinion))
}

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second
)

type FCoB struct {
	Events *FCoBEvents

	tangle *Tangle

	opinionStorage           *objectstorage.ObjectStorage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
}

func NewFCoB(store kvstore.KVStore, tangle *Tangle) (fcob *FCoB) {
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

func (f *FCoB) Vote() *events.Event {
	return f.Events.Vote
}

func (f *FCoB) VoteError() *events.Event {
	return f.Events.Error
}

func (f *FCoB) Opinion(messageID MessageID) (opinion bool) {
	f.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			opinion = f.OpinionEssence(transactionID).liked
		}
	})
	return
}

func (f *FCoB) Setup(payloadEvent *events.Event) {
	f.Events.PayloadOpinionFormed = payloadEvent
}

func (f *FCoB) Evaluate(messageID MessageID) {
	f.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			f.onTransactionBooked(transactionID, messageID)
			return
		}
		// likes by default all non-value-transaction messages
		f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, true})
	})
}

func (f *FCoB) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
		// if the opinion has been already set by the opinion provider, re-use it
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		}
		// otherwise the PayloadOpinionFormed will be triggerd by iterating over the Attachments
		// either after FCOB or as a result of an FPC voting.
		return
	})

	opinion := &Opinion{
		transactionID: transactionID,
	}
	var timestamp time.Time
	f.tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	// filters both rejected and invalid branch
	branchInclusionState := f.tangle.LedgerState.BranchInclusionState(f.tangle.LedgerState.BranchID(transactionID))
	if branchInclusionState == ledgerstate.Rejected {
		opinion.OpinionEssence = OpinionEssence{
			timestamp:        timestamp,
			levelOfKnowledge: Two,
		}
		f.opinionStorage.Store(opinion).Release()
		return
	}

	if f.tangle.LedgerState.TransactionConflicting(transactionID) {
		opinion.OpinionEssence = deriveOpinion(timestamp, f.OpinionsEssence(f.tangle.LedgerState.ConflictSet(transactionID)))

		cachedOpinion := f.opinionStorage.Store(opinion)
		defer cachedOpinion.Release()

		if opinion.LevelOfKnowledge() == One {
			//trigger voting for this transactionID
			vote := voter.Dislike
			if opinion.liked {
				vote = voter.Like
			}
			f.Events.Vote.Trigger(transactionID.String(), vote) // TODO: fix opinion type
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
			if f.tangle.LedgerState.TransactionConflicting(transactionID) {
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
				if f.tangle.LedgerState.TransactionConflicting(transactionID) {
					//trigger voting for this transactionID
					f.Events.Vote.Trigger(transactionID.String(), voter.Like)
					return
				}
				opinion.SetLevelOfKnowledge(Two)
				// trigger OpinionPayloadFormed
				messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
				for _, messageID := range messageIDs {
					f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
				}
			})
		}, timestamp.Add(LocallyFinalizedThreshold))
	}, timestamp.Add(LikedThreshold))
}

// ProcessVote allows an external voter to hand in the results of the voting process.
func (o *FCoB) ProcessVote(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			o.Events.Error.Trigger(err)

			return
		}

		o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == voter.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			messageIDs := o.tangle.Storage.AttachmentMessageIDs(transactionID)
			for _, messageID := range messageIDs {
				o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
			}
		})
	}
}

func (o *FCoB) OpinionsEssence(conflictSet ledgerstate.TransactionIDs) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, 0)
	for conflictID := range conflictSet {
		opinions = append(opinions, o.OpinionEssence(conflictID))
	}
	return
}

// func (o *FCoB) conflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
// 	return o.tangle.LedgerState.ConflictSet(transactionID)
// }

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
