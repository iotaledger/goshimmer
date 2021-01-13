package dashboard

import (
	"runtime"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysis "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	liked    = 1
	disliked = 2
)

var (
	fpcLiveFeedWorkerCount     = 1
	fpcLiveFeedWorkerQueueSize = 300
	fpcLiveFeedWorkerPool      *workerpool.WorkerPool

	fpcStoreFinalizedWorkerCount     = runtime.GOMAXPROCS(0)
	fpcStoreFinalizedWorkerQueueSize = 300
	fpcStoreFinalizedWorkerPool      *workerpool.WorkerPool

	activeConflicts = newActiveConflictSet()
)

// FPCUpdate contains an FPC update.
type FPCUpdate struct {
	Conflicts conflictSet `json:"conflictset" bson:"conflictset"`
}

func configureFPCLiveFeed() {

	if config.Node().Bool(CfgMongoDBEnabled) {
		mongoDB()
	}

	fpcLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newMsg := task.Param(0).(*FPCUpdate)
		broadcastWsMessage(&wsmsg{MsgTypeFPC, newMsg}, true)
		task.Return(nil)
	}, workerpool.WorkerCount(fpcLiveFeedWorkerCount), workerpool.QueueSize(fpcLiveFeedWorkerQueueSize))

	if config.Node().Bool(CfgMongoDBEnabled) {
		fpcStoreFinalizedWorkerPool = workerpool.New(func(task workerpool.Task) {
			storeFinalizedVoteContext(task.Param(0).(FPCRecords))
			task.Return(nil)
		}, workerpool.WorkerCount(fpcStoreFinalizedWorkerCount), workerpool.QueueSize(fpcStoreFinalizedWorkerQueueSize))
	}
}

func runFPCLiveFeed() {
	if err := daemon.BackgroundWorker("Analysis[FPCUpdater]", func(shutdownSignal <-chan struct{}) {
		onFPCHeartbeatReceived := events.NewClosure(func(hb *packet.FPCHeartbeat) {
			fpcLiveFeedWorkerPool.Submit(createFPCUpdate(hb))
		})
		analysis.Events.FPCHeartbeat.Attach(onFPCHeartbeatReceived)

		fpcLiveFeedWorkerPool.Start()
		defer fpcLiveFeedWorkerPool.Stop()

		if config.Node().Bool(CfgMongoDBEnabled) {
			fpcStoreFinalizedWorkerPool.Start()
			defer fpcStoreFinalizedWorkerPool.Stop()
		}

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

func createFPCUpdate(hb *packet.FPCHeartbeat) *FPCUpdate {
	// prepare the update
	conflicts := make(conflictSet)
	nodeID := analysis.ShortNodeIDString(hb.OwnID)
	for ID, context := range hb.RoundStats.ActiveVoteContexts {
		newVoteContext := voteContext{
			NodeID:   nodeID,
			Rounds:   context.Rounds,
			Opinions: opinion.ConvertOpinionsToInts32(context.Opinions),
		}

		conflicts[ID] = newConflict()
		conflicts[ID].NodesView[nodeID] = newVoteContext

		// update recorded events
		activeConflicts.update(ID, conflict{NodesView: map[string]voteContext{nodeID: newVoteContext}})
	}

	// check finalized conflicts
	if len(hb.Finalized) == 0 {
		return &FPCUpdate{Conflicts: conflicts}
	}

	finalizedConflicts := make(FPCRecords, len(hb.Finalized))
	i := 0
	for ID, finalOpinion := range hb.Finalized {
		conflictOverview, ok := activeConflicts.load(ID)
		if !ok {
			log.Error("Error: missing conflict with ID:", ID)
			continue
		}
		conflictDetail := conflictOverview.NodesView[nodeID]
		conflictDetail.Outcome = opinion.ConvertOpinionToInt32(finalOpinion)
		conflicts[ID] = newConflict()
		conflicts[ID].NodesView[nodeID] = conflictDetail
		activeConflicts.update(ID, conflicts[ID])
		finalizedConflicts[i] = FPCRecord{
			ConflictID: ID,
			NodeID:     conflictDetail.NodeID,
			Rounds:     conflictDetail.Rounds,
			Opinions:   conflictDetail.Opinions,
			Outcome:    conflictDetail.Outcome,
			Time:       primitive.NewDateTimeFromTime(time.Now()),
		}
		i++

		metrics.Events().AnalysisFPCFinalized.Trigger(&metrics.AnalysisFPCFinalizedEvent{
			ConflictID: ID,
			NodeID:     conflictDetail.NodeID,
			Rounds:     conflictDetail.Rounds,
			Opinions:   opinion.ConvertInts32ToOpinions(conflictDetail.Opinions),
			Outcome:    opinion.ConvertInt32Opinion(conflictDetail.Outcome),
		})
	}

	if config.Node().Bool(CfgMongoDBEnabled) {
		fpcStoreFinalizedWorkerPool.TrySubmit(finalizedConflicts)
	}

	return &FPCUpdate{Conflicts: conflicts}
}

// stores the given finalized vote contexts into the database.
func storeFinalizedVoteContext(finalizedConflicts FPCRecords) {
	db := mongoDB()

	if err := pingOrReconnectMongoDB(); err != nil {
		return
	}

	if err := storeFPCRecords(finalizedConflicts, db); err != nil {
		log.Errorf("Error while writing on MongoDB: %s", err)
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
