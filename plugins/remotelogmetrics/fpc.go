package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func onVoteFinalized(ev *vote.OpinionEvent) {
	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}
	record := &remotelogmetrics.FPCConflictRecord{
		Type:                 "fpc",
		ConflictID:           ev.ID,
		NodeID:               nodeID,
		Rounds:               ev.Ctx.Rounds,
		Opinions:             opinion.ConvertOpinionsToInts32ForLiveFeed(ev.Ctx.Opinions),
		Outcome:              opinion.ConvertOpinionToInt32ForLiveFeed(ev.Opinion),
		Time:                 clock.SyncedTime(),
		ConflictCreationTime: ev.Ctx.ConflictCreationTime,
		Delta:                clock.Since(ev.Ctx.ConflictCreationTime).Nanoseconds(),
	}
	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send FPC conflict record on vote finalized event", "err", err)
	}
}

func onVoteRoundExecuted(roundStats *vote.RoundStats) {
	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}
	for conflictID, conflictContext := range roundStats.ActiveVoteContexts {
		record := &remotelogmetrics.FPCConflictRecord{
			Type:                 "fpc",
			ConflictID:           conflictID,
			NodeID:               nodeID,
			Rounds:               conflictContext.Rounds,
			Opinions:             opinion.ConvertOpinionsToInts32ForLiveFeed(conflictContext.Opinions),
			Time:                 clock.SyncedTime(),
			ConflictCreationTime: conflictContext.ConflictCreationTime,
			Delta:                clock.Since(conflictContext.ConflictCreationTime).Nanoseconds(),
		}
		if err := deps.RemoteLogger.Send(record); err != nil {
			Plugin.Logger().Errorw("Failed to send FPC conflict record on round executed event", "err", err)
		}
	}
}
