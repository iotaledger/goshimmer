package fcob

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
)

var (
	LikedThreshold = 5 * time.Second

	LocallyFinalizedThreshold = 10 * time.Second
)

type Manager struct {
	utxoDAG        *ledgerstate.UTXODAG
	branchDAG      *ledgerstate.BranchDAG
	opinionStorage *objectstorage.ObjectStorage
}

func NewManager(store kvstore.KVStore, utxoDAG *ledgerstate.UTXODAG, branchDAG *ledgerstate.BranchDAG) (manager *Manager) {
	manager = &Manager{
		utxoDAG:   utxoDAG,
		branchDAG: branchDAG,
	}

	return
}

func (m *Manager) Opinion(transactionID ledgerstate.TransactionID) (opinion *Opinion) {
	(&CachedOpinion{CachedObject: m.opinionStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) objectstorage.StorableObject {
		return m.deriveOpinion(transactionID)
	})}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion
	})

	return
}

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
