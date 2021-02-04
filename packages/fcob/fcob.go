package fcob

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/timedexecutor"
)

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second

	onMessageBooked = events.NewClosure(func(cachedMessageEvent *tangle.CachedMessageEvent) {})
)

type Manager struct {
	utxoDAG        *ledgerstate.UTXODAG
	branchDAG      *ledgerstate.BranchDAG
	opinionStorage *objectstorage.ObjectStorage

	likedThresholdExecutor *timedexecutor.TimedExecutor

	locallyFinalizedExecutor *timedexecutor.TimedExecutor

	events Events
}

func NewManager(store kvstore.KVStore, utxoDAG *ledgerstate.UTXODAG, branchDAG *ledgerstate.BranchDAG) (manager *Manager) {
	manager = &Manager{
		utxoDAG:                  utxoDAG,
		branchDAG:                branchDAG,
		likedThresholdExecutor:   timedexecutor.New(1),
		locallyFinalizedExecutor: timedexecutor.New(1),
		events: Events{
			PayloadOpinionFormed: events.NewEvent(payloadOpinionCaller),
		},
	}

	return
}

func (m *Manager) flow(transactionID ledgerstate.TransactionID) {

	// if the opinion for this transactionID is already present,
	// it's a reattachment and thus, we re-use the same opinion.
	if opinion := m.Opinion(transactionID); opinion != nil {
		if opinion.LevelOfKnowledge > One {
			// trigger PayloadOpinionFormedEvent
		}
		return
	}

	var opinion *Opinion
	var timestamp time.Time

	m.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		timestamp = transactionMetadata.SolidificationTime()
	})

	if m.isConflicting(transactionID) {
		opinion = deriveOpinion(timestamp, m.Opinions(m.conflictSet(transactionID)))
		if opinion != nil {
			opinion.transactionID = transactionID
			m.opinionStorage.Store(opinion).Release()
			if opinion.LevelOfKnowledge == One {
				//trigger voting for this transactionID
			}
		}

		return
	}

	opinion = &Opinion{
		transactionID:    transactionID,
		Timestamp:        timestamp,
		LevelOfKnowledge: Pending,
	}
	m.opinionStorage.Store(opinion).Release()

	// Wait LikedThreshold
	m.likedThresholdExecutor.ExecuteAt(func() {
		if m.isConflicting(transactionID) {
			opinion = &Opinion{
				transactionID:    transactionID,
				Timestamp:        timestamp,
				Liked:            false,
				LevelOfKnowledge: One,
			}
			m.opinionStorage.Store(opinion).Release()
			//trigger voting for this transactionID
		}

		opinion = &Opinion{
			transactionID:    transactionID,
			Timestamp:        timestamp,
			Liked:            true,
			LevelOfKnowledge: One,
		}
		m.opinionStorage.Store(opinion).Release()

		// Wait LocallyFinalizedThreshold
		m.locallyFinalizedExecutor.ExecuteAt(func() {
			if m.isConflicting(transactionID) {
				// trigger voting?
			}

			opinion = &Opinion{
				transactionID:    transactionID,
				Timestamp:        timestamp,
				Liked:            true,
				LevelOfKnowledge: Two,
			}
			m.opinionStorage.Store(opinion).Release()
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
	cachedTransactionMetadata := m.utxoDAG.TransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	return transactionMetadata.BranchID() == ledgerstate.NewBranchID(transactionID)
}

func (m *Manager) conflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	m.branchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		m.branchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
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
	m.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		if transactionMetadata.Finalized() {
			opinion = &Opinion{
				Liked:            true,
				LevelOfKnowledge: Three,
			}
			return
		}

		if transactionMetadata.BranchID() != ledgerstate.NewBranchID(transactionID) {
			if transactionMetadata.SolidificationTime().Add(LocallyFinalizedThreshold).Before(clock.SyncedTime()) {
				opinion = &Opinion{
					Liked:            true,
					LevelOfKnowledge: Two,
				}
				return
			}

			if transactionMetadata.SolidificationTime().Add(LikedThreshold).Before(clock.SyncedTime()) {
				opinion = &Opinion{
					Liked:            true,
					LevelOfKnowledge: One,
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
			Timestamp:        targetTime,
			Liked:            false,
			LevelOfKnowledge: Two,
		}
		return
	}

	anchor := conflictSet.anchor()
	if anchor == nil {
		opinion = &Opinion{
			Timestamp:        targetTime,
			LevelOfKnowledge: Pending,
		}
		return
	}

	opinion = &Opinion{
		Timestamp:        targetTime,
		Liked:            false,
		LevelOfKnowledge: One,
	}
	return
}
