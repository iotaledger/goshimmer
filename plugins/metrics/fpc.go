package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/vote"
)

var activeConflicts uint64
var finalizedConflictCount uint64
var failedConflictCount uint64
var averageRoundsToFinalize float64
var avLock syncutils.RWMutex

// ActiveConflicts returns the number of currently active conflicts.
func ActiveConflicts() uint64 {
	return atomic.LoadUint64(&activeConflicts)
}

// FinalizedConflict returns the number of finalized conflicts since the start of the node.
func FinalizedConflict() uint64 {
	return atomic.LoadUint64(&finalizedConflictCount)
}

// FailedConflicts returns the number of failed conflicts since the start of the node.
func FailedConflicts() uint64 {
	return atomic.LoadUint64(&failedConflictCount)
}

// AverageRoundsToFinalize returns the average number fo rounds it takes to finalize conflicts since the start of the node.
func AverageRoundsToFinalize() float64 {
	avLock.RLock()
	defer avLock.RUnlock()
	return averageRoundsToFinalize
}

//// logic broken into "process..."  functions to be able to write unit tests ////

func processRoundStats(stats vote.RoundStats) {
	// get the number of active conflicts
	numActive := (uint64)(len(stats.ActiveVoteContexts))
	atomic.StoreUint64(&activeConflicts, numActive)
}

func processFinalized(ctx vote.Context) {
	avLock.Lock()
	defer avLock.Unlock()
	// calculate sum of all rounds, including the currently finalized
	sumRounds := averageRoundsToFinalize*(float64)(atomic.LoadUint64(&finalizedConflictCount)) + (float64)(ctx.Rounds)
	// increase finalized counter
	atomic.AddUint64(&finalizedConflictCount, 1)
	// calculate new average
	averageRoundsToFinalize = sumRounds / (float64)(atomic.LoadUint64(&finalizedConflictCount))
}

func processFailed(ctx vote.Context) {
	atomic.AddUint64(&failedConflictCount, 1)
}
