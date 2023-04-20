package dashboardmetrics

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/types"
)

var (
	// total number of conflicts in the database at startup.
	initialConflictTotalCountDB uint64

	// total number of finalized conflicts in the database at startup.
	initialFinalizedConflictCountDB uint64

	// total number of confirmed conflicts in the database at startup.
	initialConfirmedConflictCountDB uint64

	// number of conflicts created since the node started.
	conflictTotalCountDB atomic.Uint64

	// number of conflicts finalized since the node started.
	finalizedConflictCountDB atomic.Uint64

	// current number of confirmed conflicts.
	confirmedConflictCount atomic.Uint64

	// total time it took all conflicts to finalize. unit is milliseconds!
	conflictConfirmationTotalTime atomic.Uint64

	// all active conflicts stored in this map, to avoid duplicated event triggers for conflict confirmation.
	activeConflicts map[utxo.TransactionID]types.Empty

	activeConflictsMutex sync.RWMutex
)

// ConflictConfirmationTotalTime returns total time it took for all confirmed conflicts to be confirmed.
func ConflictConfirmationTotalTime() uint64 {
	return conflictConfirmationTotalTime.Load()
}

// ConfirmedConflictCount returns the number of confirmed conflicts.
func ConfirmedConflictCount() uint64 {
	return initialConfirmedConflictCountDB + confirmedConflictCount.Load()
}

// TotalConflictCountDB returns the total number of conflicts.
func TotalConflictCountDB() uint64 {
	return initialConflictTotalCountDB + conflictTotalCountDB.Load()
}

// FinalizedConflictCountDB returns the number of non-confirmed conflicts.
func FinalizedConflictCountDB() uint64 {
	return initialFinalizedConflictCountDB + finalizedConflictCountDB.Load()
}

func addActiveConflict(conflictID utxo.TransactionID) (added bool) {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()

	if _, exists := activeConflicts[conflictID]; !exists {
		activeConflicts[conflictID] = types.Void
		return true
	}

	return false
}

func removeActiveConflict(conflictID utxo.TransactionID) (removed bool) {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()

	if _, exists := activeConflicts[conflictID]; exists {
		delete(activeConflicts, conflictID)
		return true
	}

	return false
}

func measureInitialConflictStats() {
	activeConflictsMutex.Lock()
	defer activeConflictsMutex.Unlock()
	activeConflicts = make(map[utxo.TransactionID]types.Empty)
	conflictsToRemove := make([]utxo.TransactionID, 0)

	deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ForEachConflict(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		switch conflict.ID() {
		case utxo.EmptyTransactionID:
			return
		default:
			initialConflictTotalCountDB++
			activeConflicts[conflict.ID()] = types.Void
			if deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().ConfirmationState(utxo.NewTransactionIDs(conflict.ID())).IsAccepted() {
				conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
					if conflictingConflict.ID() != conflict.ID() {
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
		if c, exists := deps.Protocol.Engine().Ledger.MemPool().ConflictDAG().Conflict(conflictID); exists {
			c.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
				if conflictingConflict.ID() != conflictID {
					delete(activeConflicts, conflictingConflict.ID())
				}
				return true
			})
		}
		delete(activeConflicts, conflictID)
	}
}
