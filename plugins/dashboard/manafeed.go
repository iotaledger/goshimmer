package dashboard

import (
	"time"

	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
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
	Time      int64   `json:"time"`
}

type manaNetworkList struct {
	ManaType string            `json:"manaType"`
	Nodes    []manaPkg.NodeStr `json:"nodes"`
}

func configureManaFeed() {
	manaFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch task.Param(0).(byte) {
		case MsgTypeManaValue:
			sendManaValue()
		case MsgTypeManaMapOverall:
			sendManaMapOverall()
		case MsgTypeManaMapOnline:
			sendManaMapOnline()
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
			Time:      time.Now().Unix(),
		},
	})
}

func sendManaMapOverall() {
	accessManaList := mana.GetHighestManaNodes(manaPkg.AccessMana, 100)
	accessPayload := manaNetworkList{ManaType: manaPkg.AccessMana.String()}
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
	}
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOverall,
		Data: accessPayload,
	})
	consensusManaList := mana.GetHighestManaNodes(manaPkg.ConsensusMana, 100)
	consensusPayload := manaNetworkList{ManaType: manaPkg.ConsensusMana.String()}
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
	}
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOverall,
		Data: consensusPayload,
	})
}

func sendManaMapOnline() {
	accessManaList, _ := mana.GetOnlineNodes(manaPkg.AccessMana)
	accessPayload := manaNetworkList{ManaType: manaPkg.AccessMana.String()}
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
	}
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOnline,
		Data: accessPayload,
	})
	consensusManaList, _ := mana.GetOnlineNodes(manaPkg.AccessMana)
	consensusPayload := manaNetworkList{ManaType: manaPkg.ConsensusMana.String()}
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
	}
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOnline,
		Data: consensusPayload,
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
				manaFeedWorkerPool.Submit(MsgTypeManaMapOverall)
				manaFeedWorkerPool.Submit(MsgTypeManaMapOnline)
			}
		}

	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
