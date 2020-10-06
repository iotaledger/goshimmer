package dashboard

import (
	"time"

	"github.com/gorilla/websocket"
	manaPkg "github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/mana"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
)

var (
	manaFeedWorkerCount     = 1
	manaFeedWorkerQueueSize = 50
	manaFeedWorkerPool      *workerpool.WorkerPool
)

func configureManaFeed() {
	manaFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch task.Param(0).(byte) {
		case MsgTypeManaValue:
			sendManaValue()
		case MsgTypeManaMapOverall:
			sendManaMapOverall()
		case MsgTypeManaMapOnline:
			sendManaMapOnline()
		case MsgTypeManaPledge:
			sendManaPledge(task.Param(1).(*manaPkg.PledgedEvent))
		case MsgTypeManaRevoke:
			sendManaRevoke(task.Param(1).(*manaPkg.RevokedEvent))
		}
		task.Return(nil)
	}, workerpool.WorkerCount(manaFeedWorkerCount), workerpool.QueueSize(manaFeedWorkerQueueSize))
}

func runManaFeed() {
	notifyManaPledge := events.NewClosure(func(ev *manaPkg.PledgedEvent) {
		manaFeedWorkerPool.Submit(MsgTypeManaPledge, ev)
	})
	notifyManaRevoke := events.NewClosure(func(ev *manaPkg.RevokedEvent) {
		manaFeedWorkerPool.Submit(MsgTypeManaRevoke, ev)
	})
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(shutdownSignal <-chan struct{}) {
		manaPkg.Events().Pledged.Attach(notifyManaPledge)
		manaPkg.Events().Revoked.Attach(notifyManaRevoke)
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

//region Websocket message sending handlers (live updates)
func sendManaValue() {
	ownID := local.GetInstance().ID()
	access, _ := mana.GetAccessMana(ownID)
	consensus, _ := mana.GetConsensusMana(ownID)
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaValue,
		Data: &manaValueMsgData{
			NodeID:    ownID.String(),
			Access:    access,
			Consensus: consensus,
			Time:      time.Now().Unix(),
		},
	})
}

func sendManaMapOverall() {
	accessManaList := mana.GetHighestManaNodes(manaPkg.AccessMana, 0)
	accessPayload := manaNetworkListMsgData{ManaType: manaPkg.AccessMana.String()}
	totalAccessMana := 0.0
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOverall,
		Data: accessPayload,
	})
	consensusManaList := mana.GetHighestManaNodes(manaPkg.ConsensusMana, 0)
	consensusPayload := manaNetworkListMsgData{ManaType: manaPkg.ConsensusMana.String()}
	totalConsensusMana := 0.0
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
		totalConsensusMana += consensusManaList[i].Mana
	}
	consensusPayload.TotalMana = totalConsensusMana
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOverall,
		Data: consensusPayload,
	})
}

func sendManaMapOnline() {
	accessManaList, _ := mana.GetOnlineNodes(manaPkg.AccessMana)
	accessPayload := manaNetworkListMsgData{ManaType: manaPkg.AccessMana.String()}
	totalAccessMana := 0.0
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOnline,
		Data: accessPayload,
	})
	consensusManaList, _ := mana.GetOnlineNodes(manaPkg.AccessMana)
	consensusPayload := manaNetworkListMsgData{ManaType: manaPkg.ConsensusMana.String()}
	totalConsensusMana := 0.0
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
		totalConsensusMana += consensusManaList[i].Mana
	}
	consensusPayload.TotalMana = totalConsensusMana
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaMapOnline,
		Data: consensusPayload,
	})
}

func sendManaPledge(ev *manaPkg.PledgedEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaPledge,
		Data: &manaPledgeMsgData{
			ManaType: ev.Type.String(),
			NodeID:   ev.NodeID.String(),
			Time:     ev.Time.Unix(),
			BM1:      ev.AmountBM1,
			BM2:      ev.AmountBM2,
		},
	})
}

func sendManaRevoke(ev *manaPkg.RevokedEvent) {
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaRevoke,
		Data: &manaRevokeMsgData{
			ManaType: ev.Type.String(),
			NodeID:   ev.NodeID.String(),
			Time:     ev.Time.Unix(),
			BM1:      ev.AmountBM1,
		},
	})
}

//endregion

//region Websocket message sending handlers (initial data)
func sendAllowedManaPledge(ws *websocket.Conn) error {
	allowedAccess := mana.GetAllowedPledgeNodes(manaPkg.AccessMana)
	allowedConsensus := mana.GetAllowedPledgeNodes(manaPkg.ConsensusMana)

	wsmsgData := &allowedPledgeIDsMsgData{}
	wsmsgData.Access.Enabled = allowedAccess.IsFilterEnabled
	allowedAccess.Allowed.ForEach(func(element interface{}) {
		ID := element.(identity.ID)
		wsmsgData.Access.AllowedNodeIDs = append(wsmsgData.Access.AllowedNodeIDs, allowedNodeStr{
			ShortID: ID.String(),
			FullID:  base58.Encode(ID.Bytes()),
		})
	})
	wsmsgData.Consensus.Enabled = allowedConsensus.IsFilterEnabled
	allowedConsensus.Allowed.ForEach(func(element interface{}) {
		ID := element.(identity.ID)
		wsmsgData.Consensus.AllowedNodeIDs = append(wsmsgData.Consensus.AllowedNodeIDs, allowedNodeStr{
			ShortID: ID.String(),
			FullID:  base58.Encode(ID.Bytes()),
		})
	})

	if err := sendJSON(ws, &wsmsg{
		Type: MsgTypeManaAllowedPledge,
		Data: wsmsgData,
	}); err != nil {
		return err
	}
	return nil
}

//endregion

//region Websocket message data structs
type manaValueMsgData struct {
	NodeID    string  `json:"nodeID"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
	Time      int64   `json:"time"`
}

type manaNetworkListMsgData struct {
	ManaType  string            `json:"manaType"`
	TotalMana float64           `json:"totalMana"`
	Nodes     []manaPkg.NodeStr `json:"nodes"`
}

type allowedPledgeIDsMsgData struct {
	Access    pledgeIDFilter `json:"accessFilter"`
	Consensus pledgeIDFilter `json:"consensusFilter"`
}

type pledgeIDFilter struct {
	Enabled        bool             `json:"enabled"`
	AllowedNodeIDs []allowedNodeStr `json:"allowedNodeIDs"`
}

type allowedNodeStr struct {
	ShortID string `json:"shortID"`
	FullID  string `json:"fullID"`
}

type manaPledgeMsgData struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	BM1      float64 `json:"bm1"`
	BM2      float64 `json:"bm2"`
}

type manaRevokeMsgData struct {
	ManaType string  `json:"manaType"`
	NodeID   string  `json:"nodeID"`
	Time     int64   `json:"time"`
	BM1      float64 `json:"bm1"`
}

//endregion
