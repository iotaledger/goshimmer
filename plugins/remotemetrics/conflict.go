package remotemetrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/clock"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
)

var (
	// current number of confirmed  conflicts.
	confirmedConflictCount atomic.Uint64

	// number of conflicts created since the node started.
	conflictTotalCountDB atomic.Uint64

	// number of conflicts finalized since the node started.
	finalizedConflictCountDB atomic.Uint64

	// total number of conflicts in the database at startup.
	initialConflictTotalCountDB uint64
	// total number of finalized conflicts in the database at startup.
	initialFinalizedConflictCountDB uint64

	// total number of confirmed conflicts in the database at startup.
	initialConfirmedConflictCountDB uint64

	// all active conflicts stored in this map, to avoid duplicated event triggers for conflict confirmation.
	activeConflicts      map[utxo.TransactionID]types.Empty
	activeConflictsMutex sync.Mutex
)

func onConflictConfirmed(conflictID utxo.TransactionID) {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()
	if _, exists := activeConflicts[conflictID]; !exists {
		return
	}
	transactionID := conflictID
	// update conflict metric counts even if node is not synced.
	oldestAttachmentTime, oldestAttachmentBlockID, err := updateMetricCounts(conflictID, transactionID)

	if err != nil || !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.ConflictConfirmationMetrics{
		Type:               "conflictConfirmation",
		NodeID:             nodeID,
		MetricsLevel:       Parameters.MetricsLevel,
		BlockID:            oldestAttachmentBlockID.Base58(),
		ConflictID:         conflictID.Base58(),
		CreatedTimestamp:   oldestAttachmentTime,
		ConfirmedTimestamp: clock.SyncedTime(),
		DeltaConfirmed:     clock.Since(oldestAttachmentTime).Nanoseconds(),
	}
	deps.Tangle.Storage.Block(oldestAttachmentBlockID).Consume(func(block *tangleold.Block) {
		issuerID := identity.NewID(block.IssuerPublicKey())
		record.IssuerID = issuerID.String()
	})
	_ = deps.RemoteLogger.Send(record)
	sendConflictMetrics()
}

func sendConflictMetrics() {
	if !deps.Tangle.Synced() {
		return
	}

	var myID string
	if deps.Local != nil {
		myID = deps.Local.Identity.ID().String()
	}

	record := remotemetrics.ConflictCountUpdate{
		Type:                             "conflictCounts",
		NodeID:                           myID,
		MetricsLevel:                     Parameters.MetricsLevel,
		TotalConflictCount:               conflictTotalCountDB.Load() + initialConflictTotalCountDB,
		InitialTotalConflictCount:        initialConflictTotalCountDB,
		TotalConflictCountSinceStart:     conflictTotalCountDB.Load(),
		ConfirmedConflictCount:           confirmedConflictCount.Load() + initialConfirmedConflictCountDB,
		InitialConfirmedConflictCount:    initialConfirmedConflictCountDB,
		ConfirmedConflictCountSinceStart: confirmedConflictCount.Load(),
		FinalizedConflictCount:           finalizedConflictCountDB.Load() + initialFinalizedConflictCountDB,
		InitialFinalizedConflictCount:    initialFinalizedConflictCountDB,
		FinalizedConflictCountSinceStart: finalizedConflictCountDB.Load(),
	}
	_ = deps.RemoteLogger.Send(record)
}

func updateMetricCounts(conflictID utxo.TransactionID, transactionID utxo.TransactionID) (time.Time, tangleold.BlockID, error) {
	oldestAttachmentTime, oldestAttachmentBlockID, err := deps.Tangle.Utils.FirstAttachment(transactionID)
	if err != nil {
		return time.Time{}, tangleold.BlockID{}, err
	}
	deps.Tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
		if conflictingConflictID != conflictID {
			finalizedConflictCountDB.Inc()
			delete(activeConflicts, conflictingConflictID)
		}
		return true
	})
	finalizedConflictCountDB.Inc()
	confirmedConflictCount.Inc()
	delete(activeConflicts, conflictID)
	return oldestAttachmentTime, oldestAttachmentBlockID, nil
}

func measureInitialConflictCounts() {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()
	activeConflicts = make(map[utxo.TransactionID]types.Empty)
	conflictsToRemove := make([]utxo.TransactionID, 0)
	deps.Tangle.Ledger.ConflictDAG.Utils.ForEachConflict(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		switch conflict.ID() {
		case utxo.EmptyTransactionID:
			return
		default:
			initialConflictTotalCountDB++
			activeConflicts[conflict.ID()] = types.Void
			if deps.Tangle.Ledger.ConflictDAG.ConfirmationState(utxo.NewTransactionIDs(conflict.ID())).IsAccepted() {
				deps.Tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflict.ID(), func(conflictingConflictID utxo.TransactionID) bool {
					if conflictingConflictID != conflict.ID() {
						initialFinalizedConflictCountDB++
					}
					return true
				})
				initialFinalizedConflictCountDB++
				initialConfirmedConflictCountDB++
				conflictsToRemove = append(conflictsToRemove, conflict.ID())
			}
		}
	})

	// remove finalized conflicts from the map in separate loop when all conflicting conflicts are known
	for _, conflictID := range conflictsToRemove {
		deps.Tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
			if conflictingConflictID != conflictID {
				delete(activeConflicts, conflictingConflictID)
			}
			return true
		})
		delete(activeConflicts, conflictID)
	}
}
