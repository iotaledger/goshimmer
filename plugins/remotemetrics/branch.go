package remotemetrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	// current number of confirmed  branches.
	confirmedBranchCount atomic.Uint64

	// number of branches created since the node started.
	branchTotalCountDB atomic.Uint64

	// number of branches finalized since the node started.
	finalizedBranchCountDB atomic.Uint64

	// total number of branches in the database at startup.
	initialBranchTotalCountDB uint64
	// total number of finalized branches in the database at startup.
	initialFinalizedBranchCountDB uint64

	// total number of confirmed branches in the database at startup.
	initialConfirmedBranchCountDB uint64

	// all active branches stored in this map, to avoid duplicated event triggers for branch confirmation.
	activeBranches      map[utxo.TransactionID]types.Empty
	activeBranchesMutex sync.Mutex
)

func onBranchConfirmed(branchID utxo.TransactionID) {
	activeBranchesMutex.Lock()
	defer activeBranchesMutex.Unlock()
	if _, exists := activeBranches[branchID]; !exists {
		return
	}
	transactionID := branchID
	// update branch metric counts even if node is not synced.
	oldestAttachmentTime, oldestAttachmentMessageID, err := updateMetricCounts(branchID, transactionID)

	if err != nil || !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.BranchConfirmationMetrics{
		Type:               "branchConfirmation",
		NodeID:             nodeID,
		MetricsLevel:       Parameters.MetricsLevel,
		MessageID:          oldestAttachmentMessageID.Base58(),
		BranchID:           branchID.Base58(),
		CreatedTimestamp:   oldestAttachmentTime,
		ConfirmedTimestamp: clock.SyncedTime(),
		DeltaConfirmed:     clock.Since(oldestAttachmentTime).Nanoseconds(),
	}
	deps.Tangle.Storage.Message(oldestAttachmentMessageID).Consume(func(message *tangle.Message) {
		issuerID := identity.NewID(message.IssuerPublicKey())
		record.IssuerID = issuerID.String()
	})
	_ = deps.RemoteLogger.Send(record)
	sendBranchMetrics()
}

func sendBranchMetrics() {
	if !deps.Tangle.Synced() {
		return
	}

	var myID string
	if deps.Local != nil {
		myID = deps.Local.Identity.ID().String()
	}

	record := remotemetrics.BranchCountUpdate{
		Type:                           "branchCounts",
		NodeID:                         myID,
		MetricsLevel:                   Parameters.MetricsLevel,
		TotalBranchCount:               branchTotalCountDB.Load() + initialBranchTotalCountDB,
		InitialTotalBranchCount:        initialBranchTotalCountDB,
		TotalBranchCountSinceStart:     branchTotalCountDB.Load(),
		ConfirmedBranchCount:           confirmedBranchCount.Load() + initialConfirmedBranchCountDB,
		InitialConfirmedBranchCount:    initialConfirmedBranchCountDB,
		ConfirmedBranchCountSinceStart: confirmedBranchCount.Load(),
		FinalizedBranchCount:           finalizedBranchCountDB.Load() + initialFinalizedBranchCountDB,
		InitialFinalizedBranchCount:    initialFinalizedBranchCountDB,
		FinalizedBranchCountSinceStart: finalizedBranchCountDB.Load(),
	}
	_ = deps.RemoteLogger.Send(record)
}

func updateMetricCounts(branchID utxo.TransactionID, transactionID utxo.TransactionID) (time.Time, tangle.MessageID, error) {
	oldestAttachmentTime, oldestAttachmentMessageID, err := deps.Tangle.Utils.FirstAttachment(transactionID)
	if err != nil {
		return time.Time{}, tangle.MessageID{}, err
	}
	deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID utxo.TransactionID) bool {
		if conflictingBranchID != branchID {
			finalizedBranchCountDB.Inc()
			delete(activeBranches, conflictingBranchID)
		}
		return true
	})
	finalizedBranchCountDB.Inc()
	confirmedBranchCount.Inc()
	delete(activeBranches, branchID)
	return oldestAttachmentTime, oldestAttachmentMessageID, nil
}

func measureInitialBranchCounts() {
	activeBranchesMutex.Lock()
	defer activeBranchesMutex.Unlock()
	activeBranches = make(map[utxo.TransactionID]types.Empty)
	conflictsToRemove := make([]utxo.TransactionID, 0)
	deps.Tangle.Ledger.BranchDAG.Utils.ForEachBranch(func(branch *branchdag.Branch[utxo.TransactionID, utxo.OutputID]) {
		switch branch.ID() {
		case utxo.EmptyTransactionID:
			return
		default:
			initialBranchTotalCountDB++
			activeBranches[branch.ID()] = types.Void
			branchGoF, err := deps.Tangle.Ledger.Utils.BranchGradeOfFinality(branch.ID())
			if err != nil {
				return
			}
			if branchGoF == gof.High {
				deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branch.ID(), func(conflictingBranchID utxo.TransactionID) bool {
					if conflictingBranchID != branch.ID() {
						initialFinalizedBranchCountDB++
					}
					return true
				})
				initialFinalizedBranchCountDB++
				initialConfirmedBranchCountDB++
				conflictsToRemove = append(conflictsToRemove, branch.ID())
			}
		}
	})

	// remove finalized branches from the map in separate loop when all conflicting branches are known
	for _, branchID := range conflictsToRemove {
		deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID utxo.TransactionID) bool {
			if conflictingBranchID != branchID {
				delete(activeBranches, conflictingBranchID)
			}
			return true
		})
		delete(activeBranches, branchID)
	}
}
