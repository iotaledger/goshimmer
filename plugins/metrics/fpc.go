package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/vote"
)

var activeConflicts uint64

func processRoundStats(stats vote.RoundStats) {
	// get the number of active conflicts
	numActive := (uint64)(len(stats.ActiveVoteContexts))
	atomic.StoreUint64(&activeConflicts, numActive)
}

// ActiveConflicts returns the number of currently active conflicts.
func ActiveConflicts() uint64 {
	return atomic.LoadUint64(&activeConflicts)
}
