package dashboard

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/metrics"
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
	liked    = 1
	disliked = 2
)

var (
	fpcLiveFeedWorkerCount     = 1
	fpcLiveFeedWorkerQueueSize = 300
	fpcLiveFeedWorkerPool      *workerpool.WorkerPool

	activeConflicts *activeConflictSet
)

// FPCUpdate contains an FPC update.
type FPCUpdate struct {
	Conflicts conflictSet `json:"conflictset" bson:"conflictset"`
}

func configureFPCLiveFeed() {
	activeConflicts = newActiveConflictSet()

	fpcLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newMsg := task.Param(0).(*FPCUpdate)
		//fmt.Println("broadcasting FPC message to websocket clients")
		broadcastWsMessage(&wsmsg{MsgTypeFPC, newMsg}, true)
		task.Return(nil)
	}, workerpool.WorkerCount(fpcLiveFeedWorkerCount), workerpool.QueueSize(fpcLiveFeedWorkerQueueSize))
}

func runFPCLiveFeed() {
	if err := daemon.BackgroundWorker("Analysis[FPCUpdater]", func(shutdownSignal <-chan struct{}) {
		onFPCHeartbeatReceived := events.NewClosure(func(hb *packet.FPCHeartbeat) {
			//fmt.Println("broadcasting FPC live feed")
			fpcLiveFeedWorkerPool.Submit(createFPCUpdate(hb, true))
		})
		analysis.Events.FPCHeartbeat.Attach(onFPCHeartbeatReceived)

		fpcLiveFeedWorkerPool.Start()
		defer fpcLiveFeedWorkerPool.Stop()

		cleanUpTicker := time.NewTicker(1 * time.Minute)

		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping Analysis[FPCUpdater] ...")
				analysis.Events.FPCHeartbeat.Detach(onFPCHeartbeatReceived)
				cleanUpTicker.Stop()
				log.Info("Stopping Analysis[FPCUpdater] ... done")
				return
			case <-cleanUpTicker.C:
				log.Debug("Cleaning up Finalized Conflicts ...")
				activeConflicts.cleanUp()
				log.Debug("Cleaning up Finalized Conflicts ... done")
			}
		}
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func createFPCUpdate(hb *packet.FPCHeartbeat, recordEvent bool) *FPCUpdate {
	// prepare the update
	conflicts := make(conflictSet)
	nodeID := base58.Encode(hb.OwnID)
	for ID, context := range hb.RoundStats.ActiveVoteContexts {
		newVoteContext := voteContext{
			NodeID:   nodeID,
			Rounds:   context.Rounds,
			Opinions: vote.ConvertOpinionsToInts32(context.Opinions),
		}

		conflicts[ID] = newConflict()
		conflicts[ID].NodesView[nodeID] = newVoteContext

		if recordEvent {
			// update recorded events
			activeConflicts.update(ID, conflict{NodesView: map[string]voteContext{nodeID: newVoteContext}})
		}
	}

	if recordEvent {
		// check finalized conflicts
		if len(hb.Finalized) > 0 {
			finalizedConflicts := make([]FPCRecord, len(hb.Finalized))
			i := 0
			for ID, finalOpinion := range hb.Finalized {
				conflictOverview, ok := activeConflicts.load(ID)
				if !ok {
					log.Error("Error: missing conflict with ID:", ID)
					continue
				}
				conflictDetail := conflictOverview.NodesView[nodeID]
				conflictDetail.Outcome = vote.ConvertOpinionToInt32(finalOpinion)
				conflicts[ID] = newConflict()
				conflicts[ID].NodesView[nodeID] = conflictDetail
				activeConflicts.update(ID, conflicts[ID])
				finalizedConflicts[i] = FPCRecord{
					ConflictID: ID,
					NodeID:     conflictDetail.NodeID,
					Rounds:     conflictDetail.Rounds,
					Opinions:   conflictDetail.Opinions,
					Outcome:    conflictDetail.Outcome,
				}
				i++

				metrics.Events().AnalysisFPCFinalized.Trigger(&metrics.AnalysisFPCFinalizedEvent{
					ConflictID: ID,
					NodeID:     conflictDetail.NodeID,
					Rounds:     conflictDetail.Rounds,
					Opinions:   vote.ConvertInts32ToOpinions(conflictDetail.Opinions),
					Outcome:    vote.ConvertInt32Opinion(conflictDetail.Outcome),
				})
			}

			err := storeFPCRecords(finalizedConflicts, mongoDB())
			if err != nil {
				log.Errorf("Error while writing on MongoDB: %s", err)
			}
		}
	}

	return &FPCUpdate{
		Conflicts: conflicts,
	}
}

// replay FPC records (past events).
func replayFPCRecords(ws *websocket.Conn) {
	wsMessage := &wsmsg{MsgTypeFPC, activeConflicts.ToFPCUpdate()}

	if err := ws.WriteJSON(wsMessage); err != nil {
		log.Info(err)
		return
	}
	if err := ws.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout)); err != nil {
		return
	}
}
