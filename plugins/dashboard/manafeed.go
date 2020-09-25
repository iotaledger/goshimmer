package dashboard

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	manaFeedWorkerCount     = 1
	manaFeedWorkerQueueSize = 50
	manaFeedWorkerPool      *workerpool.WorkerPool
)

type manaValue struct {
	NodeID    string  `json:"nodeID"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
	Time      string  `json:"time"`
}

type manaNetworkList struct {
	Nodes []manaValue `json:"nodes"`
}

func configureManaFeed() {
	manaFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch task.Param(0).(byte) {
		case MsgTypeManaValue:
			sendManaValue()
		case MsgTypeManaMapOverall:
		case MsgTypeManaMapOnline:
		}
		task.Return(nil)
	}, workerpool.WorkerCount(manaFeedWorkerCount), workerpool.QueueSize(manaFeedWorkerQueueSize))
}

func sendManaValue() {
	ownID := local.GetInstance().ID()
	access, _ := mana.GetAccessMana(ownID)
	consensus, _ := mana.GetConsensusMana(ownID)
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaValue,
		Data: &manaValue{
			NodeID:    ownID.String(),
			Access:    access,
			Consensus: consensus,
			Time:      time.Now().String(),
		},
	})
}

func runManaFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(shutdownSignal <-chan struct{}) {
		manaFeedWorkerPool.Start()
		manaTicker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-shutdownSignal:
				log.Info("Stopping Dashboard[ManaUpdater] ...")
				manaFeedWorkerPool.Stop()
				manaTicker.Stop()
				log.Info("Stopping Dashboard[ManaUpdater] ... done")
				return
			case <-manaTicker.C:
				manaFeedWorkerPool.Submit(MsgTypeManaValue)
			}
		}

	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
