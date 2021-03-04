package fcob

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	voter "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/timedqueue"
)

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

// region ConsensusMechanism ///////////////////////////////////////////////////////////////////////////////////////////

// ConsensusMechanism represents the FPC
type ConsensusMechanism struct {
	Events *Events

	tangle                   *tangle.Tangle
	opinionStorage           *objectstorage.ObjectStorage
	timestampOpinionStorage  *objectstorage.ObjectStorage
	likedThresholdExecutor   *timedexecutor.TimedExecutor
	locallyFinalizedExecutor *timedexecutor.TimedExecutor
	waiting                  *opinionWait
}

func NewConsensusMechanism() *ConsensusMechanism {
	return &ConsensusMechanism{
		Events: &Events{
			Error:                  events.NewEvent(events.ErrorCaller),
			Vote:                   events.NewEvent(voteEvent),
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(tangle.MessageIDEventHandler),
		},
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		waiting:                  &opinionWait{waitMap: make(map[tangle.MessageID]*waitStruct)},
	}
}

func (f *ConsensusMechanism) Init(tangle *tangle.Tangle) {
	f.tangle = tangle

	osFactory := objectstorage.NewFactory(tangle.Options.Store, database.PrefixFCOB)
	f.opinionStorage = osFactory.New(PrefixOpinion, OpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false))
	f.timestampOpinionStorage = osFactory.New(PrefixTimestampOpinion, TimestampOpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false))
}

func (f *ConsensusMechanism) Setup() {
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.Evaluate))
	f.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(f.EvaluateTimestamp))

	f.Events.PayloadOpinionFormed.Attach(events.NewClosure(f.onPayloadOpinionFormed))
}

// Evaluate evaluates the opinion of the given messageID.
func (f *ConsensusMechanism) Evaluate(messageID tangle.MessageID) {
	if f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		f.onTransactionBooked(transactionID, messageID)
	}) {
		return
	}
	// likes by default all non-value-transaction messages
	f.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, true})
}

func (f *ConsensusMechanism) EvaluateTimestamp(messageID tangle.MessageID) {
	f.StoreTimestampOpinion(&TimestampOpinion{
		MessageID: messageID,
		Value:     voter.Like,
		LoK:       Two,
	})

	f.setEligibility(messageID)

	if f.waiting.done(messageID, timestampOpinion) {
		f.tangle.OpinionManager.Events.MessageOpinionFormed.Trigger(messageID)
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

func (f *ConsensusMechanism) PayloadLiked(messageID tangle.MessageID) (liked bool) {
	return f.Opinion(messageID)
}

func (f *ConsensusMechanism) Shutdown() {
	f.likedThresholdExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.locallyFinalizedExecutor.Shutdown(timedqueue.CancelPendingElements)
	f.opinionStorage.Shutdown()
	f.timestampOpinionStorage.Shutdown()
}

// Opinion returns the liked status of a given messageID.
func (f *ConsensusMechanism) Opinion(messageID tangle.MessageID) (opinion bool) {
	f.tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		opinion = f.OpinionEssence(transactionID).liked
	})
	return
}

// TransactionOpinionEssence returns the opinion essence of a given transactionID.
func (f *ConsensusMechanism) TransactionOpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	opinion = f.OpinionEssence(transactionID)
	return
}

// TimestampOpinion returns the timestampOpinion of the given message metadata.
func (f *ConsensusMechanism) TimestampOpinion(messageID tangle.MessageID) (timestampOpinion *TimestampOpinion) {
	(&CachedTimestampOpinion{CachedObject: f.timestampOpinionStorage.Load(messageID.Bytes())}).Consume(func(opinion *TimestampOpinion) {
		timestampOpinion = opinion
	})

	return
}

// SetTimestampOpinion sets the timestampOpinion flag.
// It returns true if the timestampOpinion flag is modified. False otherwise.
func (f *ConsensusMechanism) StoreTimestampOpinion(timestampOpinion *TimestampOpinion) (modified bool) {
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
	})

	return
}

// OpinionEssence returns the OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given transactionID.
func (f *ConsensusMechanism) OpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	(&CachedOpinion{CachedObject: f.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion.OpinionEssence
	})

	return
}

// OpinionsEssence returns a list of OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given conflicSet.
func (f *ConsensusMechanism) OpinionsEssence(conflictSet ledgerstate.TransactionIDs) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, 0)
	for conflictID := range conflictSet {
		opinions = append(opinions, f.OpinionEssence(conflictID))
	}
	return
}

// CachedOpinion returns the CachedOpinion of the given TransactionID.
func (f *ConsensusMechanism) CachedOpinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
	cachedOpinion = &CachedOpinion{CachedObject: f.opinionStorage.Load(transactionID.Bytes())}
	return
}

func (f *ConsensusMechanism) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID tangle.MessageID) {
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

func (f *ConsensusMechanism) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
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

func (f *ConsensusMechanism) setEligibility(messageID tangle.MessageID) {
	f.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		timestampOpinion := f.TimestampOpinion(messageID)

		messageMetadata.SetEligible(
			timestampOpinion != nil && timestampOpinion.Value == opinion.Like && timestampOpinion.LoK > One && f.parentsEligibility(messageID),
		)
	})
}

// parentsEligibility checks if the parents are eligible.
func (f *ConsensusMechanism) parentsEligibility(messageID tangle.MessageID) (eligible bool) {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region otherstuff ///////////////////////////////////////////////////////////////////////////////////////////////////

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
