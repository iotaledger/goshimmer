package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/hive.go/syncutils"
)

var activeConflicts uint64
var finalizedConflictCount uint64
var failedConflictCount uint64
var averageRoundsToFinalize float64
var avLock syncutils.RWMutex

// QueryReceivedCount is the number of queries received (each query can contain multiple conflicts to give an opinion about)
var QueryReceivedCount uint64

// OpinionQueryReceivedCount is the number of opinion queries received (multiple in one query)
var OpinionQueryReceivedCount uint64

// QueryReplyErrorCount counts how many times we haven't received an answer for our query. (each query reply can contain multiple conflicts to get an opinion about)
var QueryReplyErrorCount uint64

// OpinionQueryReplyErrorCount counts how many opinions we asked for but never heard back (multiple opinions in one query)
var OpinionQueryReplyErrorCount uint64

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

// FPCQueryReceived returns the number of received voting queries. For an exact number of opinion queries, use FPCOpinionQueryReceived().
func FPCQueryReceived() uint64 {
	return atomic.LoadUint64(&QueryReceivedCount)
}

// FPCOpinionQueryReceived returns the number of received opinion queries.
func FPCOpinionQueryReceived() uint64 {
	return atomic.LoadUint64(&OpinionQueryReceivedCount)
}

// FPCQueryReplyErrors returns the number of sent but unanswered queries for conflict opinions. For an exact number of failed opinions, use FPCOpinionQueryReplyErrors().
func FPCQueryReplyErrors() uint64 {
	return atomic.LoadUint64(&QueryReplyErrorCount)
}

// FPCOpinionQueryReplyErrors returns the number of opinions that the node failed to gather from peers.
func FPCOpinionQueryReplyErrors() uint64 {
	return atomic.LoadUint64(&OpinionQueryReplyErrorCount)
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

func processQueryReceived(ev *metrics.QueryReceivedEvent) {
	// received one query
	atomic.AddUint64(&QueryReceivedCount, 1)
	// containing this many conflicts to give opinion about
	atomic.AddUint64(&OpinionQueryReceivedCount, (uint64)(ev.OpinionCount))
}

func processQueryReplyError(ev *metrics.QueryReplyErrorEvent) {
	// received one query
	atomic.AddUint64(&QueryReplyErrorCount, 1)
	// containing this many conflicts to give opinion about
	atomic.AddUint64(&OpinionQueryReplyErrorCount, (uint64)(ev.OpinionCount))
}
