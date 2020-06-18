package client

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
)

var (
	finalized      map[string]vote.Opinion
	finalizedMutex sync.RWMutex
)

func onFinalized(ev *vote.OpinionEvent) {
	finalizedMutex.Lock()
	finalized[ev.ID] = ev.Opinion
	finalizedMutex.Unlock()
}

func onRoundExecuted(roundStats *vote.RoundStats) {
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	chunks := splitFPCVoteContext(roundStats.ActiveVoteContexts)

	for _, chunk := range chunks {
		// abort if empty round
		if len(chunk) == 0 {
			return
		}

		rs := vote.RoundStats{
			Duration:           roundStats.Duration,
			RandUsed:           roundStats.RandUsed,
			ActiveVoteContexts: chunk,
		}

		hb := &packet.FPCHeartbeat{
			OwnID:      nodeID,
			RoundStats: rs,
		}

		finalizedMutex.Lock()
		hb.Finalized = finalized
		finalized = make(map[string]vote.Opinion)
		finalizedMutex.Unlock()

		data, err := packet.NewFPCHeartbeatMessage(hb)
		if err != nil {
			log.Info(err, " - FPC heartbeat message skipped")
			return
		}

		log.Info("Client: onRoundExecuted data size: ", len(data))

		if _, err = conn.Write(data); err != nil {
			log.Debugw("Error while writing to connection", "Description", err)
			return
		}
		// trigger AnalysisOutboundBytes event
		metrics.Events().AnalysisOutboundBytes.Trigger(uint64(len(data)))
	}
}

func splitFPCVoteContext(ctx map[string]*vote.Context) (chunk []map[string]*vote.Context) {
	chunk = make([]map[string]*vote.Context, 1)
	i, counter := 0, 0
	chunk[i] = make(map[string]*vote.Context)

	if len(ctx) < maxVoteContext {
		chunk[i] = ctx
		return
	}

	for conflictID, voteCtx := range ctx {
		counter++
		if counter >= maxVoteContext {
			counter = 0
			i++
			chunk = append(chunk, make(map[string]*vote.Context))
		}
		chunk[i][conflictID] = voteCtx
	}

	return
}
