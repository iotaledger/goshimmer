package fcob

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/timedqueue"
)

// region FCoB /////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixOpinion defines the storage prefix for the opinion storage.
	PrefixOpinion byte = iota

	// PrefixTimestampOpinion defines the storage prefix for the timestamp opinion storage.
	PrefixTimestampOpinion

	// cacheTime defines the duration that the object storage caches objects.
	cacheTime = 2 * time.Second
)

var (
	// LikedThreshold is the first time thresshold of FCoB.
	LikedThreshold = 5 * time.Second

	// LocallyFinalizedThreshold is the second time thresshold of FCoB.
	LocallyFinalizedThreshold = 10 * time.Second
)

// FCoB is the component implementing the Fast ConsensusProvider of Barcelona protocol.
type FCoB struct {
	Events *FCoBEvents

	tangle *tangle.Tangle

	opinionStorage           *objectstorage.ObjectStorage
	timestampOpinionStorage  *objectstorage.ObjectStorage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
}

// NewFCoB returns a new instance of FCoB.
func NewFCoB(tangle *tangle.Tangle) (fcob *FCoB) {
	osFactory := objectstorage.NewFactory(tangle.Options.Store, database.PrefixFCOB)
	fcob = &FCoB{
		tangle:                   tangle,
		opinionStorage:           osFactory.New(PrefixOpinion, OpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		timestampOpinionStorage:  osFactory.New(PrefixTimestampOpinion, TimestampOpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		Events: &FCoBEvents{
			Error: events.NewEvent(events.ErrorCaller),
			Vote:  events.NewEvent(voteEvent),
		},
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (f *FCoB) Setup(payloadEvent *events.Event) {
	f.Events.PayloadOpinionFormed = payloadEvent
}

// Shutdown shuts down FCoB and persists its state.
func (f *FCoB) Shutdown() {
	f.likedThresholdExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.locallyFinalizedExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.opinionStorage.Shutdown()
}

// Vote trigger a voting request.
func (f *FCoB) Vote() *events.Event {
	return f.Events.Vote
}

// VoteError notify an error coming from the result of voting.
func (f *FCoB) VoteError() *events.Event {
	return f.Events.Error
}

// Opinion returns the liked status of a given messageID.
func (f *FCoB) Opinion(messageID tangle.MessageID) (opinion bool) {
	f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		opinion = f.OpinionEssence(transactionID).liked
	})
	return
}

// TransactionOpinionEssence returns the opinion essence of a given transactionID.
func (f *FCoB) TransactionOpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	opinion = f.OpinionEssence(transactionID)
	return
}

// Evaluate evaluates the opinion of the given messageID.
func (f *FCoB) Evaluate(messageID tangle.MessageID) {
	if f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		f.onTransactionBooked(transactionID, messageID)
	}) {
		return
	}
	// likes by default all non-value-transaction messages
	f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, true})
}

func (f *FCoB) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	isReattachment := false
	f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
		// if the opinion has been already set by the opinion provider, re-use it
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		}
		// otherwise the PayloadOpinionFormed will be triggerd by iterating over the Attachments
		// either after FCOB or as a result of an FPC voting.
		isReattachment = true
	})
	if isReattachment {
		return
	}

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
			liked:            false,
			levelOfKnowledge: Two,
		}
		f.opinionStorage.Store(opinion).Release()
		f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
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
			f.Events.Vote.Trigger(transactionID.Base58(), vote)
		}

		if opinion.LevelOfKnowledge() > One {
			f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
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
				f.Events.Vote.Trigger(transactionID.Base58(), voter.Dislike)
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
					f.Events.Vote.Trigger(transactionID.Base58(), voter.Like)
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
func (f *FCoB) ProcessVote(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			f.Events.Error.Trigger(err)
			return
		}

		f.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == voter.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			messageIDs := f.tangle.Storage.AttachmentMessageIDs(transactionID)
			for _, messageID := range messageIDs {
				f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
			}
		})
	}
}

// TimestampOpinion returns the timestampOpinion of the given message metadata.
func (f *FCoB) TimestampOpinion(messageID tangle.MessageID) (timestampOpinion *TimestampOpinion) {
	(&CachedTimestampOpinion{CachedObject: f.timestampOpinionStorage.Load(messageID.Bytes())}).Consume(func(opinion *TimestampOpinion) {
		timestampOpinion = opinion
	})

	return
}

// SetTimestampOpinion sets the timestampOpinion flag.
// It returns true if the timestampOpinion flag is modified. False otherwise.
func (f *FCoB) StoreTimestampOpinion(timestampOpinion *TimestampOpinion) (modified bool) {
	cachedTimestampOpinion := &CachedTimestampOpinion{CachedObject: f.timestampOpinionStorage.ComputeIfAbsent(timestampOpinion.MessageID.Bytes(), func(key []byte) objectstorage.StorableObject {
		timestampOpinion.SetModified()
		timestampOpinion.Persist()
		modified = true

		return timestampOpinion
	})}

	if modified {
		cachedTimestampOpinion.Release()
		return
	}

	cachedTimestampOpinion.Consume(func(loadedTimestampOpinion *TimestampOpinion) {
		if loadedTimestampOpinion.Equals(timestampOpinion) {
			return
		}

		loadedTimestampOpinion.LoK = timestampOpinion.LoK
		loadedTimestampOpinion.Value = timestampOpinion.Value

		timestampOpinion.SetModified()
		timestampOpinion.Persist()
		modified = true

		return
	})

	return
}

// OpinionEssence returns the OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given transactionID.
func (f *FCoB) OpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	(&CachedOpinion{CachedObject: f.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion.OpinionEssence
	})

	return
}

// OpinionsEssence returns a list of OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given conflicSet.
func (f *FCoB) OpinionsEssence(conflictSet ledgerstate.TransactionIDs) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, 0)
	for conflictID := range conflictSet {
		opinions = append(opinions, f.OpinionEssence(conflictID))
	}
	return
}

// CachedOpinion returns the CachedOpinion of the given TransactionID.
func (f *FCoB) CachedOpinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
	cachedOpinion = &CachedOpinion{CachedObject: f.opinionStorage.Load(transactionID.Bytes())}
	return
}

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FCoBEvents /////////////////////////////////////////////////////////////////////////////////////////////

// FCoBEvents defines all the events related to the Fast ConsensusProvider of Barcelona protocol.
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
