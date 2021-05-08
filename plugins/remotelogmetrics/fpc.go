package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
	"sync"
)

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
		record := &remotelogmetrics.FPCConflictRecord{
			Type:       "fpc",
			ConflictID: conflictID,
			NodeID:     nodeID,
			Rounds:     conflictContext.Rounds,
			Opinions:   opinion.ConvertOpinionsToInts32ForLiveFeed(conflictContext.Opinions),
			Outcome:    ml.getOutcome(conflictID),
			Time:       clock.SyncedTime(),
		}
		if err := remotelog.RemoteLogger().Send(record); err != nil {
			plugin.Logger().Errorw("Failed to send FPC conflict record on round executed event", "err", err)
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
