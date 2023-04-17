package remotemetrics

import (
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
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
	activeConflicts      *advancedset.AdvancedSet[utxo.TransactionID]
	activeConflictsMutex sync.Mutex
)

func onConflictConfirmed(conflictID utxo.TransactionID) {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()
	if !activeConflicts.Has(conflictID) {
		return
	}
	transactionID := conflictID
	// update conflict metric counts even if node is not synced.
	oldestAttachment := updateMetricCounts(conflictID, transactionID)

	if !deps.Protocol.Engine().IsSynced() {
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
		BlockID:            oldestAttachment.ID().Base58(),
		ConflictID:         conflictID.Base58(),
		CreatedTimestamp:   oldestAttachment.IssuingTime(),
		ConfirmedTimestamp: time.Now(),
		DeltaConfirmed:     time.Since(oldestAttachment.IssuingTime()).Nanoseconds(),
	}
	issuerID := identity.NewID(oldestAttachment.IssuerPublicKey())
	record.IssuerID = issuerID.String()
	_ = deps.RemoteLogger.Send(record)
	sendConflictMetrics()
}

func sendConflictMetrics() {
	if !deps.Protocol.Engine().IsSynced() {
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

func updateMetricCounts(conflictID utxo.TransactionID, transactionID utxo.TransactionID) (oldestAttachment *booker.Block) {
	oldestAttachment = deps.Protocol.Engine().Tangle.Booker().GetEarliestAttachment(transactionID)
	conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
	if !exists {
		return oldestAttachment
	}
	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		finalizedConflictCountDB.Inc()
		activeConflicts.Delete(conflictingConflict.ID())
		return true
	})
	finalizedConflictCountDB.Inc()
	confirmedConflictCount.Inc()
	activeConflicts.Delete(conflictID)
	return oldestAttachment
}

func measureInitialConflictCounts() {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()
	activeConflicts = advancedset.New[utxo.TransactionID]()
	conflictsToRemove := make([]utxo.TransactionID, 0)
	deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ForEachConflict(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		switch conflict.ID() {
		case utxo.EmptyTransactionID:
			return
		default:
			initialConflictTotalCountDB++
			activeConflicts.Add(conflict.ID())
			if deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ConfirmationState(utxo.NewTransactionIDs(conflict.ID())).IsAccepted() {
				conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
					initialFinalizedConflictCountDB++
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
		conflict, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID)
		if !exists {
			continue
		}
		conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
			activeConflicts.Delete(conflictingConflict.ID())
			return true
		})
		activeConflicts.Delete(conflictID)
	}
}
