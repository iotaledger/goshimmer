package remotemetrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	// current number of confirmed  branches
	confirmedBranchCount atomic.Uint64

	// number of branches created since the node started
	branchTotalCountDB atomic.Uint64

	// number of branches finalized since the node started
	finalizedBranchCountDB atomic.Uint64

	// total number of branches in the database at startup
	initialBranchTotalCountDB uint64
	// total number of finalized branches in the database at startup
	initialFinalizedBranchCountDB uint64

	// total number of confirmed branches in the database at startup
	initialConfirmedBranchCountDB uint64
)

func onBranchConfirmed(branchID ledgerstate.BranchID) {
	transactionID := branchID.TransactionID()
	// update branch metric counts even if node is not synced
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

	if err = deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send BranchConfirmationMetrics record", "err", err)
	}
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
	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send BranchConfirmationMetrics record", "err", err)
	}
}

func updateMetricCounts(branchID ledgerstate.BranchID, transactionID ledgerstate.TransactionID) (time.Time, tangle.MessageID, error) {
	oldestAttachmentTime, oldestAttachmentMessageID, err := deps.Tangle.Utils.FirstAttachment(transactionID)
	if err != nil {
		return time.Time{}, tangle.MessageID{}, err
	}
	deps.Tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		if conflictingBranchID != branchID {
			finalizedBranchCountDB.Inc()
		}
	})
	finalizedBranchCountDB.Inc()
	confirmedBranchCount.Inc()
	return oldestAttachmentTime, oldestAttachmentMessageID, nil
}

func measureInitialBranchCounts() {
	deps.Tangle.LedgerState.BranchDAG.ForEachBranch(func(branch ledgerstate.Branch) {
		switch branch.ID() {
		case ledgerstate.MasterBranchID:
			return
		case ledgerstate.InvalidBranchID:
			return
		case ledgerstate.LazyBookedConflictsBranchID:
			return
		default:
			initialBranchTotalCountDB++
			branchGoF, err := deps.Tangle.LedgerState.UTXODAG.BranchGradeOfFinality(branch.ID())
			if err != nil {
				return
			}
			if branchGoF == gof.High {
				deps.Tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branch.ID(), func(conflictingBranchID ledgerstate.BranchID) {
					if conflictingBranchID != branch.ID() {
						initialFinalizedBranchCountDB++
					}
				})
				initialFinalizedBranchCountDB++
				initialConfirmedBranchCountDB++
			}
		}
	})
}
