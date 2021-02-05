package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
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

type Manager struct {
	tangle         *Tangle
	opinionStorage *objectstorage.ObjectStorage

	likedThresholdExecutor *timedexecutor.TimedExecutor

	locallyFinalizedExecutor *timedexecutor.TimedExecutor

	events OpinionFormenrEvents
}

func NewManager(store kvstore.KVStore, tangle *Tangle) (manager *Manager) {
	manager = &Manager{
		tangle:                   tangle,
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		events: OpinionFormenrEvents{
			PayloadOpinionFormed: events.NewEvent(payloadOpinionCaller),
		},
	}

	return
}

func (m *Manager) flow(transactionID ledgerstate.TransactionID) {

	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	if opinion := m.Opinion(transactionID); opinion != nil {
		if opinion.LevelOfKnowledge() > One {
			// trigger PayloadOpinionFormedEvent
		}
		return
	}

	var opinion *Opinion
	var timestamp time.Time

	m.tangle.Booker.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	if m.isConflicting(transactionID) {
		opinion = deriveOpinion(timestamp, m.Opinions(m.conflictSet(transactionID)))
		if opinion != nil {
			opinion.transactionID = transactionID
			m.opinionStorage.Store(opinion).Release()
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
	m.opinionStorage.Store(opinion).Release()

	// Wait LikedThreshold
	m.likedThresholdExecutor.ExecuteAt(func() {
		if m.isConflicting(transactionID) {
			opinion.SetLevelOfKnowledge(One)
			//trigger voting for this transactionID
		}

		opinion.SetLiked(true)
		opinion.SetLevelOfKnowledge(One)

		// Wait LocallyFinalizedThreshold
		m.locallyFinalizedExecutor.ExecuteAt(func() {
			if m.isConflicting(transactionID) {
				// trigger voting?
			}

			opinion.SetLiked(true)
			opinion.SetLevelOfKnowledge(Two)
		}, timestamp.Add(LocallyFinalizedThreshold))

	}, timestamp.Add(LikedThreshold))

}

func (m *Manager) Opinions(conflictSet []ledgerstate.TransactionID) (opinions []*Opinion) {
	opinions = make([]*Opinion, len(conflictSet))
	for i, conflictID := range conflictSet {
		opinions[i] = m.Opinion(conflictID)
	}
	return
}

func (m *Manager) isConflicting(transactionID ledgerstate.TransactionID) bool {
	cachedTransactionMetadata := m.tangle.Booker.utxoDAG.TransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	return transactionMetadata.BranchID() == ledgerstate.NewBranchID(transactionID)
}

func (m *Manager) conflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	m.tangle.Booker.branchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		m.tangle.Booker.branchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet = append(conflictSet, ledgerstate.TransactionID(conflictMember.BranchID()))
		})
	}

	return
}

func (m *Manager) Opinion(transactionID ledgerstate.TransactionID) (opinion *Opinion) {
	(&CachedOpinion{CachedObject: m.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion
	})

	return
}

// func (m *Manager) Opinion(transactionID ledgerstate.TransactionID) (opinion *Opinion) {
// 	(&CachedOpinion{CachedObject: m.opinionStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) objectstorage.StorableObject {
// 		return m.deriveOpinion(transactionID)
// 	})}).Consume(func(storedOpinion *Opinion) {
// 		opinion = storedOpinion
// 	})

// 	return
// }

func (m *Manager) deriveOpinion(transactionID ledgerstate.TransactionID) (opinion *Opinion) {
	m.tangle.Booker.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		if transactionMetadata.Finalized() {
			opinion = &Opinion{
				liked:            true,
				levelOfKnowledge: Three,
			}
			return
		}

		if transactionMetadata.BranchID() != ledgerstate.NewBranchID(transactionID) {
			if transactionMetadata.SolidificationTime().Add(LocallyFinalizedThreshold).Before(clock.SyncedTime()) {
				opinion = &Opinion{
					liked:            true,
					levelOfKnowledge: Two,
				}
				return
			}

			if transactionMetadata.SolidificationTime().Add(LikedThreshold).Before(clock.SyncedTime()) {
				opinion = &Opinion{
					liked:            true,
					levelOfKnowledge: One,
				}
				return
			}
		}
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
