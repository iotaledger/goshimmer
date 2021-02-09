package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/vote"
	FPCOpinion "github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
	"github.com/iotaledger/hive.go/types"
)

// Events defines all the events related to the opinion manager.
type OpinionFormerEvents struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event

	// Fired when an opinion of a timestamp is formed.
	TimestampOpinionFormed *events.Event

	// Fired when an opinion of a message is formed.
	MessageOpinionFormed *events.Event

	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote.
	Vote *events.Event
}

func voteEvent(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn FPCOpinion.Opinion))(params[0].(string), params[1].(FPCOpinion.Opinion))
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

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second

	onMessageBooked = events.NewClosure(func(cachedMessageEvent *CachedMessageEvent) {})
)

type OpinionFormer struct {
	Events OpinionFormerEvents

	tangle         *Tangle
	opinionStorage *objectstorage.ObjectStorage

	waiting *opinionWait

	likedThresholdExecutor *timedexecutor.TimedExecutor

	locallyFinalizedExecutor *timedexecutor.TimedExecutor
}

func NewOpinionFormer(store kvstore.KVStore, tangle *Tangle) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:                   tangle,
		waiting:                  &opinionWait{waitMap: make(map[MessageID]types.Empty)},
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		Events: OpinionFormerEvents{
			PayloadOpinionFormed:   events.NewEvent(payloadOpinionCaller),
			TimestampOpinionFormed: events.NewEvent(messageIDEventHandler),
			MessageOpinionFormed:   events.NewEvent(messageIDEventHandler),
			Error:                  events.NewEvent(events.ErrorCaller),
			Vote:                   events.NewEvent(voteEvent),
		},
	}

	return
}

func (o *OpinionFormer) Setup() {
	o.tangle.Events.MessageBooked.Attach(events.NewClosure(o.onMessageBooked))
	o.Events.PayloadOpinionFormed.Attach(events.NewClosure(o.onPayloadOpinionFormed))
	o.Events.TimestampOpinionFormed.Attach(events.NewClosure(o.onTimestampOpinionFormed))
}

func (o *OpinionFormer) onPayloadOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onTimestampOpinionFormed(ev *OpinionFormedEvent) {
	if o.waiting.done(ev.MessageID) {
		o.Events.MessageOpinionFormed.Trigger(ev.MessageID)
	}
}

func (o *OpinionFormer) onMessageBooked(messageID MessageID) {
	o.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		// TODO: add timestamp quality evaluation.
		// For now we set all timestamps as good.
		o.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetTimestampOpinion(TimestampOpinion{
				Value: FPCOpinion.Like,
				LoK:   Two,
			})
			o.Events.TimestampOpinionFormed.Trigger(messageID)
		})

		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			o.onTransactionBooked(transactionID, messageID)
			return
		}
		// likes by default all non-value-transaction messages
		o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, true})
	})
}

func (o *OpinionFormer) onTransactionBooked(transactionID ledgerstate.TransactionID, messageID MessageID) {
	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormed event
			o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		}
		return
	})

	opinion := &Opinion{
		transactionID: transactionID,
	}
	var timestamp time.Time
	o.tangle.Booker.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	if o.isConflicting(transactionID) {
		opinion.OpinionEssence = deriveOpinion(timestamp, o.Opinions(o.conflictSet(transactionID)))

		cachedOpinion := o.opinionStorage.Store(opinion)
		defer cachedOpinion.Release()

		if opinion.LevelOfKnowledge() == One {
			//trigger voting for this transactionID
			o.Events.Vote.Trigger(transactionID.String(), opinion.liked) // TODO: fix opinion type
		}

		return
	}

	opinion.OpinionEssence = OpinionEssence{
		timestamp:        timestamp,
		levelOfKnowledge: Pending,
	}
	cachedOpinion := o.opinionStorage.Store(opinion)
	defer cachedOpinion.Release()

	// Wait LikedThreshold
	o.likedThresholdExecutor.ExecuteAt(func() {
		o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLevelOfKnowledge(One)
			if o.isConflicting(transactionID) {
				opinion.SetLiked(false)
				//trigger voting for this transactionID
				o.Events.Vote.Trigger(transactionID.String(), FPCOpinion.Dislike)
				return
			}
			opinion.SetLiked(true)
		})

		// Wait LocallyFinalizedThreshold
		o.locallyFinalizedExecutor.ExecuteAt(func() {
			o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
				opinion.SetLiked(true)
				if o.isConflicting(transactionID) {
					//trigger voting for this transactionID
					o.Events.Vote.Trigger(transactionID.String(), FPCOpinion.Like)
					return
				}
				opinion.SetLevelOfKnowledge(Two)
				// trigger OpinionPayloadFormed
				o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
			})
		}, timestamp.Add(LocallyFinalizedThreshold))
	}, timestamp.Add(LikedThreshold))
}

// ProcessVoteResult allows an external voter to hand in the results of the voting process.
func (o *OpinionFormer) ProcessVoteResult(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := ledgerstate.TransactionIDFromBase58(ev.ID)
		if err != nil {
			o.Events.Error.Trigger(err)

			return
		}

		// TODO: Check monotonicity and consistency among the conflict set.

		o.CachedOpinion(transactionID).Consume(func(opinion *Opinion) {
			opinion.SetLiked(ev.Opinion == FPCOpinion.Like)
			opinion.SetLevelOfKnowledge(Two)
			// trigger PayloadOpinionFormed event
			// o.Events.PayloadOpinionFormed.Trigger(&OpinionFormedEvent{messageID, opinion.liked})
		})
	}
}

func (o *OpinionFormer) Opinions(conflictSet []ledgerstate.TransactionID) (opinions []OpinionEssence) {
	opinions = make([]OpinionEssence, len(conflictSet))
	for i, conflictID := range conflictSet {
		opinions[i] = o.Opinion(conflictID)
	}
	return
}

func (o *OpinionFormer) isConflicting(transactionID ledgerstate.TransactionID) bool {
	cachedTransactionMetadata := o.tangle.Booker.utxoDAG.TransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	return transactionMetadata.BranchID() == ledgerstate.NewBranchID(transactionID)
}

func (o *OpinionFormer) conflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	o.tangle.Booker.branchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		o.tangle.Booker.branchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet = append(conflictSet, ledgerstate.TransactionID(conflictMember.BranchID()))
		})
	}

	return
}

func (o *OpinionFormer) Opinion(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	(&CachedOpinion{CachedObject: o.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion.OpinionEssence
	})

	return
}

func (o *OpinionFormer) CachedOpinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
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
