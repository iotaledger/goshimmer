package dashboard

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var fpcLiveFeedWorkerCount = 1
var fpcLiveFeedWorkerQueueSize = 50
var fpcLiveFeedWorkerPool *workerpool.WorkerPool

type fpcRoundMsg struct {
	ID      string `json:"id"`
	Opinion int    `json:"opinion"`
}

func configureFPCLiveFeed() {
	fpcLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newMsg := task.Param(0).(vote.RoundStats)

		for _, conflict := range newMsg.ActiveVoteContexts {
			broadcastWsMessage(&wsmsg{MsgTypeDrng, &fpcRoundMsg{
				ID:      conflict.ID,
				Opinion: int(conflict.LastOpinion()),
			}})
		}
		task.Return(nil)
	}, workerpool.WorkerCount(fpcLiveFeedWorkerCount), workerpool.QueueSize(fpcLiveFeedWorkerQueueSize))
}

func runFPCLiveFeed() {
	daemon.BackgroundWorker("Analysis[FPCUpdater]", func(shutdownSignal <-chan struct{}) {
		newMsgRateLimiter := time.NewTicker(time.Second / 10)
		defer newMsgRateLimiter.Stop()

		notifyNewFPCRound := events.NewClosure(func(message vote.RoundStats) {
			select {
			case <-newMsgRateLimiter.C:
				fpcLiveFeedWorkerPool.TrySubmit(message)
			default:
			}
		})
		drng.Instance().Events.Randomness.Attach(notifyNewFPCRound)

		fpcLiveFeedWorkerPool.Start()
		defer fpcLiveFeedWorkerPool.Stop()

		<-shutdownSignal
		log.Info("Stopping Analysis[FPCUpdater] ...")
		drng.Instance().Events.Randomness.Detach(notifyNewFPCRound)
		log.Info("Stopping Analysis[FPCUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
