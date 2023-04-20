package dashboardmetrics

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/types"
)

var (
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
	return confirmedConflictCount.Load()
}

// TotalConflictCountDB returns the total number of conflicts.
func TotalConflictCountDB() uint64 {
	return conflictTotalCountDB.Load()
}

// FinalizedConflictCountDB returns the number of non-confirmed conflicts.
func FinalizedConflictCountDB() uint64 {
	return finalizedConflictCountDB.Load()
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
