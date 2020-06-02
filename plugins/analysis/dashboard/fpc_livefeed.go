package dashboard

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysis "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
)

var (
	fpcLiveFeedWorkerCount     = 1
	fpcLiveFeedWorkerQueueSize = 50
	fpcLiveFeedWorkerPool      *workerpool.WorkerPool
)

// Conflict defines the struct for the opinions of the nodes regarding a given conflict.
type Conflict struct {
	NodesView map[string]voteContext `json:"nodesview"`
}

type voteContext struct {
	NodeID   string  `json:"nodeid"`
	Rounds   int     `json:"rounds"`
	Opinions []int32 `json:"opinions"`
	Like     int32   `json:"like"`
}

// FPCMsg contains an FPC update
type FPCMsg struct {
	Nodes       int                 `json:"nodes"`
	ConflictSet map[string]Conflict `json:"conflictset"`
}

func configureFPCLiveFeed() {
	fpcLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newMsg := task.Param(0).(*FPCMsg)
		broadcastWsMessage(&wsmsg{MsgTypeFPC, newMsg})
		task.Return(nil)
	}, workerpool.WorkerCount(fpcLiveFeedWorkerCount), workerpool.QueueSize(fpcLiveFeedWorkerQueueSize))
}

func runFPCLiveFeed() {
	if err := daemon.BackgroundWorker("Analysis[FPCUpdater]", func(shutdownSignal <-chan struct{}) {
		newMsgRateLimiter := time.NewTicker(time.Second / 10)
		defer newMsgRateLimiter.Stop()

		onFPCHeartbeatReceived := events.NewClosure(func(hb *packet.FPCHeartbeat) {
			select {
			case <-newMsgRateLimiter.C:
				fpcLiveFeedWorkerPool.TrySubmit(createFPCUpdate(hb))
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
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func createFPCUpdate(hb *packet.FPCHeartbeat) *FPCMsg {
	update := make(map[string]Conflict)

	for ID, context := range hb.RoundStats.ActiveVoteContexts {
		update[ID] = newConflict()
		nodeID := base58.Encode(hb.OwnID)
		update[ID].NodesView[nodeID] = voteContext{
			NodeID:   nodeID,
			Rounds:   context.Rounds,
			Opinions: vote.ConvertOpinionsToInts32(context.Opinions),
		}
	}

	return &FPCMsg{
		ConflictSet: update,
	}
}

func newConflict() Conflict {
	return Conflict{
		NodesView: make(map[string]voteContext),
	}
}
