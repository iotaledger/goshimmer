package dashboard

import (
	"encoding/hex"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var drngLiveFeedWorkerCount = 1
var drngLiveFeedWorkerQueueSize = 50
var drngLiveFeedWorkerPool *workerpool.WorkerPool

func configureDrngLiveFeed() {
	drngLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newRandomness := task.Param(0).(state.Randomness)

		sendToAllWSClient(&wsmsg{MsgTypeDrng, &drngMsg{
			Instance:      drng.Instance.State.Committee().InstanceID,
			DistributedPK: hex.EncodeToString(drng.Instance.State.Committee().DistributedPK),
			Round:         newRandomness.Round,
			Randomness:    hex.EncodeToString(newRandomness.Randomness[:32]),
			Timestamp:     newRandomness.Timestamp.Format("2 Jan 2006 15:04:05")}})

		task.Return(nil)
	}, workerpool.WorkerCount(drngLiveFeedWorkerCount), workerpool.QueueSize(drngLiveFeedWorkerQueueSize))
}

func runDrngLiveFeed() {
	newMsgRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewRandomness := events.NewClosure(func(message state.Randomness) {
		select {
		case <-newMsgRateLimiter.C:
			drngLiveFeedWorkerPool.TrySubmit(message)
		default:
		}
	})

	daemon.BackgroundWorker("Dashboard[DRNGUpdater]", func(shutdownSignal <-chan struct{}) {
		drng.Instance.Events.Randomness.Attach(notifyNewRandomness)
		drngLiveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[DRNGUpdater] ...")
		drng.Instance.Events.Randomness.Detach(notifyNewRandomness)
		newMsgRateLimiter.Stop()
		drngLiveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[DRNGUpdater] ... done")
	}, shutdown.PriorityDashboard)
}
