package metrics

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

var (
	activeConflicts        atomic.Uint64
	finalizedConflictCount atomic.Uint64
	failedConflictCount    atomic.Uint64
	sumRounds              atomic.Uint64
	avLock                 syncutils.RWMutex

	// queryReceivedCount is the number of queries received (each query can contain multiple conflicts to give an opinion about).
	queryReceivedCount atomic.Uint64

	// opinionQueryReceivedCount is the number of opinion queries received (multiple in one query).
	opinionQueryReceivedCount atomic.Uint64

	// queryReplyErrorCount counts how many times we haven't received an answer for our query.
	// (each query reply can contain multiple conflicts to get an opinion about).
	queryReplyErrorCount atomic.Uint64

	// opinionQueryReplyErrorCount counts how many opinions we asked for but never heard back (multiple opinions in one query).
	opinionQueryReplyErrorCount atomic.Uint64
)

// ActiveConflicts returns the number of currently active conflicts.
func ActiveConflicts() uint64 {
	return activeConflicts.Load()
}

// FinalizedConflict returns the number of finalized conflicts since the start of the node.
func FinalizedConflict() uint64 {
	return finalizedConflictCount.Load()
}

// FailedConflicts returns the number of failed conflicts since the start of the node.
func FailedConflicts() uint64 {
	return failedConflictCount.Load()
}

// AverageRoundsToFinalize returns the average number of rounds it takes to finalize conflicts since the start of the node.
func AverageRoundsToFinalize() float64 {
	if FinalizedConflict() == 0 {
		return 0
	}
	return float64(sumRounds.Load()) / float64(FinalizedConflict())
}

// FPCQueryReceived returns the number of received voting queries. For an exact number of opinion queries, use FPCOpinionQueryReceived().
func FPCQueryReceived() uint64 {
	return queryReceivedCount.Load()
}

// FPCOpinionQueryReceived returns the number of received opinion queries.
func FPCOpinionQueryReceived() uint64 {
	return opinionQueryReceivedCount.Load()
}

// FPCQueryReplyErrors returns the number of sent but unanswered queries for conflict opinions. For an exact number of failed opinions, use FPCOpinionQueryReplyErrors().
func FPCQueryReplyErrors() uint64 {
	return queryReplyErrorCount.Load()
}

// FPCOpinionQueryReplyErrors returns the number of opinions that the node failed to gather from peers.
func FPCOpinionQueryReplyErrors() uint64 {
	return opinionQueryReplyErrorCount.Load()
}

//// logic broken into "process..."  functions to be able to write unit tests ////

func processRoundStats(stats *vote.RoundStats) {
	// get the number of active conflicts
	numActive := (uint64)(len(stats.ActiveVoteContexts))
	activeConflicts.Store(numActive)
}

func processFinalized(ctx vote.Context) {
	avLock.Lock()
	defer avLock.Unlock()
	// calculate sum of all rounds, including the currently finalized
	sumRounds.Add(uint64(ctx.Rounds))
	// increase finalized counter
	finalizedConflictCount.Inc()
}

func processFailed(_ vote.Context) {
	failedConflictCount.Inc()
}

func processQueryReceived(ev *metrics.QueryReceivedEvent) {
	// received one query
	queryReceivedCount.Inc()
	// containing this many conflicts to give opinion about
	opinionQueryReceivedCount.Add((uint64)(ev.OpinionCount))
}

func processQueryReplyError(ev *metrics.QueryReplyErrorEvent) {
	// received one query
	queryReplyErrorCount.Inc()
	// containing this many conflicts to give opinion about
	opinionQueryReplyErrorCount.Add((uint64)(ev.OpinionCount))
}

// FPCConflictRecord defines the FPC conflict record to sent be to remote logger.
type FPCConflictRecord struct {
	// ConflictID defines the ID of the conflict.
	ConflictID string `json:"conflictid" bson:"conflictid"`
	// NodeID defines the ID of the node.
	NodeID string `json:"nodeid" bson:"nodeid"`
	// Rounds defines number of rounds performed to finalize the conflict.
	Rounds int `json:"rounds" bson:"rounds"`
	// Opinions contains the opinion of each round.
	Opinions []int32 `json:"opinions" bson:"opinions"`
	// Outcome defines final opinion of the conflict.
	Outcome int32 `json:"outcome,omitempty" bson:"outcome,omitempty"`
	// Time defines the time when the conflict has been finalized.
	Time time.Time `json:"datetime" bson:"datetime"`
}

type fpcMetricsLogger struct {
	finalized      map[string]opinion.Opinion
	finalizedMutex sync.RWMutex
}

func newFPCMetricsLogger() *fpcMetricsLogger {
	return &fpcMetricsLogger{
		finalized: map[string]opinion.Opinion{},
	}
}

func (ml *fpcMetricsLogger) onVoteFinalized(ev *vote.OpinionEvent) {
	ml.finalizedMutex.Lock()
	defer ml.finalizedMutex.Unlock()
	ml.finalized[ev.ID] = ev.Opinion
}

func (ml *fpcMetricsLogger) onVoteRoundExecuted(roundStats *vote.RoundStats) {
	var nodeID string
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}
	for conflictID, conflictContext := range roundStats.ActiveVoteContexts {
		record := &FPCConflictRecord{
			ConflictID: conflictID,
			NodeID:     nodeID,
			Rounds:     conflictContext.Rounds,
			Opinions:   opinion.ConvertOpinionsToInts32ForLiveFeed(conflictContext.Opinions),
			Outcome:    ml.getOutcome(conflictID),
			Time:       time.Now().UTC(),
		}
		if err := remotelog.RemoteLogger().Send(record); err != nil {
			log.Errorw("Failed to send FPC conflict record on round executed event", "err", err)
		}
	}
	ml.refreshFinalized()
}

func (ml *fpcMetricsLogger) getOutcome(conflictID string) int32 {
	ml.finalizedMutex.RLock()
	defer ml.finalizedMutex.RUnlock()
	finalOpinion, ok := ml.finalized[conflictID]
	if ok {
		return opinion.ConvertOpinionToInt32ForLiveFeed(finalOpinion)
	}
	return 0
}

func (ml *fpcMetricsLogger) refreshFinalized() {
	ml.finalizedMutex.Lock()
	defer ml.finalizedMutex.Unlock()
	ml.finalized = map[string]opinion.Opinion{}
}
