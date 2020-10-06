package dashboard

import (
	"encoding/hex"
	"time"

	drngpkg "github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var drngLiveFeedWorkerCount = 1
var drngLiveFeedWorkerQueueSize = 50
var drngLiveFeedWorkerPool *workerpool.WorkerPool

type drngMsg struct {
	Instance      uint32 `json:"instance"`
	DistributedPK string `json:"dpk"`
	Round         uint64 `json:"round"`
	Randomness    string `json:"randomness"`
	Timestamp     string `json:"timestamp"`
}

func configureDrngLiveFeed() {
	drngLiveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		newRandomness := task.Param(0).(*drngpkg.State)

		broadcastWsMessage(&wsmsg{MsgTypeDrng, &drngMsg{
			Instance:      newRandomness.Committee().InstanceID,
			DistributedPK: hex.EncodeToString(newRandomness.Committee().DistributedPK),
			Round:         newRandomness.Randomness().Round,
			Randomness:    hex.EncodeToString(newRandomness.Randomness().Randomness[:32]),
			Timestamp:     newRandomness.Randomness().Timestamp.Format("2 Jan 2006 15:04:05")}})

		task.Return(nil)
	}, workerpool.WorkerCount(drngLiveFeedWorkerCount), workerpool.QueueSize(drngLiveFeedWorkerQueueSize))
}

func runDrngLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[DRNGUpdater]", func(shutdownSignal <-chan struct{}) {
		newMsgRateLimiter := time.NewTicker(time.Second / 10)
		defer newMsgRateLimiter.Stop()

		notifyNewRandomness := events.NewClosure(func(message *drngpkg.State) {
			select {
			case <-newMsgRateLimiter.C:
				drngLiveFeedWorkerPool.TrySubmit(message)
			default:
			}
		})
		drng.Instance().Events.Randomness.Attach(notifyNewRandomness)

		drngLiveFeedWorkerPool.Start()
		defer drngLiveFeedWorkerPool.Stop()

		<-shutdownSignal
		log.Info("Stopping Dashboard[DRNGUpdater] ...")
		drng.Instance().Events.Randomness.Detach(notifyNewRandomness)
		log.Info("Stopping Dashboard[DRNGUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
