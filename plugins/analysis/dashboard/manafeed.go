package dashboard

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	analysis "github.com/iotaledger/goshimmer/plugins/analysis/server"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	manaFeedWorkerCount     = 1
	manaFeedWorkerQueueSize = 50
	manaFeedWorkerPool      *workerpool.WorkerPool
	manaBuffer              *dashboard.ManaBuffer
)

func configureManaFeed() {
	manaFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch task.Param(0).(byte) {
		case dashboard.MsgTypeManaValue:
			sendManaValue(task.Param(1).(map[mana.Type]mana.NodeMap))
		case dashboard.MsgTypeManaMapOverall:
			sendManaMapOverall(task.Param(1).(map[mana.Type]mana.NodeMap))
		case dashboard.MsgTypeManaMapOnline:
			sendManaMapOnline(task.Param(1).(map[mana.Type][]mana.Node))
		case dashboard.MsgTypeManaPledge:
			sendManaPledge(task.Param(1).(mana.PledgedEvent))
		case dashboard.MsgTypeManaRevoke:
			sendManaRevoke(task.Param(1).(mana.RevokedEvent))
		}
		task.Return(nil)
	}, workerpool.WorkerCount(manaFeedWorkerCount), workerpool.QueueSize(manaFeedWorkerQueueSize))
}

func runManaFeed() {
	if err := daemon.BackgroundWorker("Analysis[ManaUpdater]", func(shutdownSignal <-chan struct{}) {
		manaBuffer = dashboard.NewManaBuffer()
		manaFeedWorkerPool.Start()
		onManaHeartbeatReceived := events.NewClosure(func(hb *packet.ManaHeartbeat) {
			createSubmit(hb)
		})
		analysis.Events.ManaHeartbeat.Attach(onManaHeartbeatReceived)

		for {
			<-shutdownSignal
			log.Info("Stopping Analysis[ManaUpdater] ...")
			analysis.Events.ManaHeartbeat.Detach(onManaHeartbeatReceived)
			manaFeedWorkerPool.Stop()
			log.Info("Stopping Analysis[ManaUpdater] ... done")
			return

		}
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func createSubmit(hb *packet.ManaHeartbeat) {
	manaFeedWorkerPool.Submit(dashboard.MsgTypeManaValue, hb.NetworkMap)
	manaFeedWorkerPool.Submit(dashboard.MsgTypeManaMapOverall, hb.NetworkMap)
	manaFeedWorkerPool.Submit(dashboard.MsgTypeManaMapOnline, hb.OnlineNetworkMap)
	for _, ev := range hb.PledgeEvents {
		manaFeedWorkerPool.Submit(dashboard.MsgTypeManaPledge, ev)
	}
	for _, ev := range hb.RevokeEvents {
		manaFeedWorkerPool.Submit(dashboard.MsgTypeManaRevoke, ev)
	}
}

func sendManaValue(allMana map[mana.Type]mana.NodeMap) {
	accessList := allMana[mana.AccessMana].ToNodeStrList()
	var totalAccess float64
	for _, v := range accessList {
		totalAccess += v.Mana
	}

	consensusList := allMana[mana.ConsensusMana].ToNodeStrList()
	var totalConsensus float64
	for _, v := range consensusList {
		totalConsensus += v.Mana
	}

	msgData := &dashboard.ManaValueMsgData{
		NodeID:    "N/A",
		Access:    totalAccess,
		Consensus: totalConsensus,
		Time:      time.Now().Unix(),
	}
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaValue,
		Data: msgData,
	})
	manaBuffer.StoreValueMsg(msgData)
}

func sendManaMapOverall(allMana map[mana.Type]mana.NodeMap) {
	accessPayload := &dashboard.ManaNetworkListMsgData{ManaType: mana.AccessMana.String()}
	totalAccessMana := 0.0
	accessNodes := allMana[mana.AccessMana].ToNodeStrList()
	for i := 0; i < len(accessNodes); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessNodes[i])
		totalAccessMana += accessNodes[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaMapOverall,
		Data: accessPayload,
	})

	consensusManaList := allMana[mana.ConsensusMana].ToNodeStrList()
	consensusPayload := &dashboard.ManaNetworkListMsgData{ManaType: mana.ConsensusMana.String()}
	totalConsensusMana := 0.0
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i])
		totalConsensusMana += consensusManaList[i].Mana
	}
	consensusPayload.TotalMana = totalConsensusMana
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaMapOverall,
		Data: consensusPayload,
	})
	manaBuffer.StoreMapOverall(accessPayload, consensusPayload)
}

func sendManaMapOnline(online map[mana.Type][]mana.Node) {
	accessManaList := online[mana.AccessMana]
	accessPayload := &dashboard.ManaNetworkListMsgData{ManaType: mana.AccessMana.String()}
	totalAccessMana := 0.0
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaMapOnline,
		Data: accessPayload,
	})

	consensusManaList := online[mana.ConsensusMana]
	consensusPayload := &dashboard.ManaNetworkListMsgData{ManaType: mana.ConsensusMana.String()}
	totalConsensusMana := 0.0
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
		totalConsensusMana += consensusManaList[i].Mana
	}
	consensusPayload.TotalMana = totalConsensusMana
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaMapOnline,
		Data: consensusPayload,
	})
	manaBuffer.StoreMapOnline(accessPayload, consensusPayload)
}

func sendManaPledge(ev mana.PledgedEvent) {
	manaBuffer.StoreEvent(&ev)
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaPledge,
		Data: ev.ToJSONSerializable(),
	})
}

func sendManaRevoke(ev mana.RevokedEvent) {
	manaBuffer.StoreEvent(&ev)
	broadcastWsMessage(&wsmsg{
		Type: dashboard.MsgTypeManaRevoke,
		Data: ev.ToJSONSerializable(),
	})
}
