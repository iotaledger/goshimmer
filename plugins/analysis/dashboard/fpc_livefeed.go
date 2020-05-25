package dashboard

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysis "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58/base58"
)

const (
	unfinalized = 0
	liked       = 1
	disliked    = 2
)

var (
	ErrConflictMissing = fmt.Errorf("conflictID missing")
)

var (
	fpcLiveFeedWorkerCount     = 1
	fpcLiveFeedWorkerQueueSize = 300
	fpcLiveFeedWorkerPool      *workerpool.WorkerPool

	recordedConflicts *conflictRecord
)

// FPCUpdate contains an FPC update.
type FPCUpdate struct {
	Conflicts ConflictSet `json:"conflictset"`
}

func configureFPCLiveFeed() {
	recordedConflicts = NewConflictRecord(100)

	fpcLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newMsg := task.Param(0).(*FPCUpdate)
		broadcastWsMessage(&wsmsg{MsgTypeFPC, newMsg})
		task.Return(nil)
	}, workerpool.WorkerCount(fpcLiveFeedWorkerCount), workerpool.QueueSize(fpcLiveFeedWorkerQueueSize))
}

func runFPCLiveFeed() {
	daemon.BackgroundWorker("Analysis[FPCUpdater]", func(shutdownSignal <-chan struct{}) {
		newMsgRateLimiter := time.NewTicker(time.Millisecond)
		defer newMsgRateLimiter.Stop()

		onFPCHeartbeatReceived := events.NewClosure(func(hb *packet.FPCHeartbeat) {
			select {
			case <-newMsgRateLimiter.C:
				fpcLiveFeedWorkerPool.TrySubmit(createFPCUpdate(hb, true))
			default:
			}
		})
		analysis.Events.FPCHeartbeat.Attach(onFPCHeartbeatReceived)

		fpcLiveFeedWorkerPool.Start()
		defer fpcLiveFeedWorkerPool.Stop()

		<-shutdownSignal
		log.Info("Stopping Analysis[FPCUpdater] ...")
		analysis.Events.FPCHeartbeat.Detach(onFPCHeartbeatReceived)
		log.Info("Stopping Analysis[FPCUpdater] ... done")
	}, shutdown.PriorityDashboard)
}

func createFPCUpdate(hb *packet.FPCHeartbeat, recordEvent bool) *FPCUpdate {
	// prepare the update
	conflicts := make(map[string]Conflict)
	nodeID := base58.Encode(hb.OwnID)
	for ID, context := range hb.RoundStats.ActiveVoteContexts {
		newVoteContext := voteContext{
			NodeID:   nodeID,
			Rounds:   context.Rounds,
			Opinions: vote.ConvertOpinionsToInts32(context.Opinions),
		}

		// check conflict has been finalized
		if _, ok := hb.Finalized[ID]; ok {
			newVoteContext.Status = vote.ConvertOpinionToInt32(hb.Finalized[ID])
		}

		conflicts[ID] = newConflict()
		conflicts[ID].NodesView[nodeID] = newVoteContext

		if recordEvent {
			// update recorded events
			recordedConflicts.Update(ID, Conflict{NodesView: map[string]voteContext{nodeID: newVoteContext}})
		}
	}

	return &FPCUpdate{
		Conflicts: conflicts,
	}
}
