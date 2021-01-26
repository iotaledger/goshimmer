package client

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/identity"
)

var (
	finalized      map[string]opinion.Opinion
	finalizedMutex sync.RWMutex
)

func onFinalized(ev *vote.OpinionEvent) {
	finalizedMutex.Lock()
	finalized[ev.ID] = ev.Opinion
	finalizedMutex.Unlock()
}

func onRoundExecuted(roundStats *vote.RoundStats) {
	// get own ID
	nodeID := make([]byte, len(identity.ID{}))
	if local.GetInstance() != nil {
		copy(nodeID, local.GetInstance().ID().Bytes())
	}

	chunks := splitFPCVoteContext(roundStats.ActiveVoteContexts)

	for _, chunk := range chunks {
		rs := vote.RoundStats{
			Duration:           roundStats.Duration,
			RandUsed:           roundStats.RandUsed,
			ActiveVoteContexts: chunk,
		}

		hb := &packet.FPCHeartbeat{
			Version:    banner.SimplifiedAppVersion,
			OwnID:      nodeID,
			RoundStats: rs,
		}

		finalizedMutex.Lock()
		hb.Finalized = finalized
		finalized = make(map[string]opinion.Opinion)
		finalizedMutex.Unlock()

		// abort if empty round and no finalized conflicts.
		if len(chunk) == 0 && len(hb.Finalized) == 0 {
			return
		}

		data, err := packet.NewFPCHeartbeatMessage(hb)
		if err != nil {
			log.Debugw("FPC heartbeat message skipped", "error", err)
			return
		}

		log.Debugw("Client: onRoundExecuted data size", "len", len(data))

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

	if len(ctx) < voteContextChunkThreshold {
		chunk[i] = ctx
		return
	}

	for conflictID, voteCtx := range ctx {
		counter++
		if counter >= voteContextChunkThreshold {
			counter = 0
			i++
			chunk = append(chunk, make(map[string]*vote.Context))
		}
		chunk[i][conflictID] = voteCtx
	}

	return
}
