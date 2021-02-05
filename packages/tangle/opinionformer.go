package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
)

// Events defines all the events related to the opinion manager.
type OpinionFormenrEvents struct {
	// Fired when an opinion of a payload is formed.
	PayloadOpinionFormed *events.Event
}

// PayloadOpinionFormedEvent holds data about a PayloadOpinionFormed event.
type PayloadOpinionFormedEvent struct {
	// The messageID of the message containing the payload.
	MessageID MessageID
	// The opinion of the payload.
	Opinion *Opinion
}

func payloadOpinionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*PayloadOpinionFormedEvent))(params[0].(*PayloadOpinionFormedEvent))
}

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second

	onMessageBooked = events.NewClosure(func(cachedMessageEvent *CachedMessageEvent) {})
)

type OpinionFormer struct {
	tangle         *Tangle
	opinionStorage *objectstorage.ObjectStorage

	likedThresholdExecutor *timedexecutor.TimedExecutor

	locallyFinalizedExecutor *timedexecutor.TimedExecutor

	events OpinionFormenrEvents
}

func NewOpinionFormer(store kvstore.KVStore, tangle *Tangle) (opinionFormer *OpinionFormer) {
	opinionFormer = &OpinionFormer{
		tangle:                   tangle,
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		events: OpinionFormenrEvents{
			PayloadOpinionFormed: events.NewEvent(payloadOpinionCaller),
		},
	}

	return
}

func (o *OpinionFormer) run(transactionID ledgerstate.TransactionID) {

	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	if opinion := o.Opinion(transactionID); opinion != nil {
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormedEvent
		}
		return
	}

	var opinion *Opinion
	var timestamp time.Time

	o.tangle.Booker.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	if o.isConflicting(transactionID) {
		opinion = deriveOpinion(timestamp, o.Opinions(o.conflictSet(transactionID)))
		if opinion != nil {
			opinion.transactionID = transactionID
			o.opinionStorage.Store(opinion).Release()
			if opinion.LevelOfKnowledge() == One {
				//trigger voting for this transactionID
			}
		}

		return
	}

	opinion = &Opinion{
		transactionID:    transactionID,
		timestamp:        timestamp,
		levelOfKnowledge: Pending,
	}
	o.opinionStorage.Store(opinion).Release()

	// Wait LikedThreshold
	o.likedThresholdExecutor.ExecuteAt(func() {
		if o.isConflicting(transactionID) {
			opinion.SetLevelOfKnowledge(One)
			//trigger voting for this transactionID
		}

		opinion.SetLiked(true)
		opinion.SetLevelOfKnowledge(One)

		// Wait LocallyFinalizedThreshold
		o.locallyFinalizedExecutor.ExecuteAt(func() {
			if o.isConflicting(transactionID) {
				// trigger voting?
			}

			opinion.SetLiked(true)
			opinion.SetLevelOfKnowledge(Two)
		}, timestamp.Add(LocallyFinalizedThreshold))

	}, timestamp.Add(LikedThreshold))

}

func (o *OpinionFormer) Opinions(conflictSet []ledgerstate.TransactionID) (opinions []*Opinion) {
	opinions = make([]*Opinion, len(conflictSet))
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

func (o *OpinionFormer) Opinion(transactionID ledgerstate.TransactionID) (opinion *Opinion) {
	(&CachedOpinion{CachedObject: o.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion
	})

	return
}

func deriveOpinion(targetTime time.Time, conflictSet ConflictSet) (opinion *Opinion) {
	if conflictSet.hasDecidedLike() {
		opinion = &Opinion{
			timestamp:        targetTime,
			liked:            false,
			levelOfKnowledge: Two,
		}
		return
	}

	anchor := conflictSet.anchor()
	if anchor == nil {
		opinion = &Opinion{
			timestamp:        targetTime,
			levelOfKnowledge: Pending,
		}
		return
	}

	opinion = &Opinion{
		timestamp:        targetTime,
		liked:            false,
		levelOfKnowledge: One,
	}
	return
}
