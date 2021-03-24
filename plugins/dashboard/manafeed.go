package dashboard

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	manaFeedWorkerCount     = 1
	manaFeedWorkerQueueSize = 50
	manaFeedWorkerPool      *workerpool.WorkerPool
	manaBuffer              *ManaBuffer
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
			sendManaPledge(task.Param(1).(*mana.PledgedEvent))
		case MsgTypeManaRevoke:
			sendManaRevoke(task.Param(1).(*mana.RevokedEvent))
		}
		task.Return(nil)
	}, workerpool.WorkerCount(manaFeedWorkerCount), workerpool.QueueSize(manaFeedWorkerQueueSize))
}

func runManaFeed() {
	notifyManaPledge := events.NewClosure(func(ev *mana.PledgedEvent) {
		manaFeedWorkerPool.Submit(MsgTypeManaPledge, ev)
	})
	notifyManaRevoke := events.NewClosure(func(ev *mana.RevokedEvent) {
		manaFeedWorkerPool.Submit(MsgTypeManaRevoke, ev)
	})
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(shutdownSignal <-chan struct{}) {
		manaBuffer = NewManaBuffer()
		mana.Events().Pledged.Attach(notifyManaPledge)
		mana.Events().Revoked.Attach(notifyManaRevoke)
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

// region Websocket message sending handlers (live updates)
func sendManaValue() {
	ownID := local.GetInstance().ID()
	access, _, err := manaPlugin.GetAccessMana(ownID)
	// if node not found, returned value is 0.0
	if err != nil && !xerrors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get own access mana: %s ", err.Error())
	}
	consensus, _, err := manaPlugin.GetConsensusMana(ownID)
	// if node not found, returned value is 0.0
	if err != nil && !xerrors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get own consensus mana: %s ", err.Error())
	}
	msgData := &ManaValueMsgData{
		NodeID:    ownID.String(),
		Access:    access,
		Consensus: consensus,
		Time:      time.Now().Unix(),
	}
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaValue,
		Data: msgData,
	})
	manaBuffer.StoreValueMsg(msgData)
}

func sendManaMapOverall() {
	accessManaList, _, err := manaPlugin.GetHighestManaNodes(mana.AccessMana, 0)
	if err != nil && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest access mana nodes: %s ", err.Error())
	}
	accessPayload := &ManaNetworkListMsgData{ManaType: mana.AccessMana.String()}
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
	consensusManaList, _, err := manaPlugin.GetHighestManaNodes(mana.ConsensusMana, 0)
	if err != nil && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest consensus mana nodes: %s ", err.Error())
	}
	consensusPayload := &ManaNetworkListMsgData{ManaType: mana.ConsensusMana.String()}
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
	manaBuffer.StoreMapOverall(accessPayload, consensusPayload)
}

func sendManaMapOnline() {
	accessManaList, _, err := manaPlugin.GetOnlineNodes(mana.AccessMana)
	if err != nil && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of online access mana nodes: %s", err.Error())
	}
	accessPayload := &ManaNetworkListMsgData{ManaType: mana.AccessMana.String()}
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
	consensusManaList, _, err := manaPlugin.GetOnlineNodes(mana.ConsensusMana)
	if err != nil && !xerrors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of online consensus mana nodes: %s ", err.Error())
	}
	consensusPayload := &ManaNetworkListMsgData{ManaType: mana.ConsensusMana.String()}
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
	manaBuffer.StoreMapOnline(accessPayload, consensusPayload)
}

func sendManaPledge(ev *mana.PledgedEvent) {
	manaBuffer.StoreEvent(ev)
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaPledge,
		Data: ev.ToJSONSerializable(),
	})
}

func sendManaRevoke(ev *mana.RevokedEvent) {
	manaBuffer.StoreEvent(ev)
	broadcastWsMessage(&wsmsg{
		Type: MsgTypeManaRevoke,
		Data: ev.ToJSONSerializable(),
	})
}

// endregion

// region Websocket message sending handlers (initial data)
func sendAllowedManaPledge(ws *websocket.Conn) error {
	allowedAccess := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	allowedConsensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)

	wsmsgData := &AllowedPledgeIDsMsgData{}
	wsmsgData.Access.Enabled = allowedAccess.IsFilterEnabled
	allowedAccess.Allowed.ForEach(func(element interface{}) {
		ID := element.(identity.ID)
		wsmsgData.Access.AllowedNodeIDs = append(wsmsgData.Access.AllowedNodeIDs, AllowedNodeStr{
			ShortID: ID.String(),
			FullID:  base58.Encode(ID.Bytes()),
		})
	})
	wsmsgData.Consensus.Enabled = allowedConsensus.IsFilterEnabled
	allowedConsensus.Allowed.ForEach(func(element interface{}) {
		ID := element.(identity.ID)
		wsmsgData.Consensus.AllowedNodeIDs = append(wsmsgData.Consensus.AllowedNodeIDs, AllowedNodeStr{
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

// endregion

// region Websocket message data structs

// ManaValueMsgData contains mana values for a particular node.
type ManaValueMsgData struct {
	NodeID    string  `json:"nodeID"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
	Time      int64   `json:"time"`
}

// ManaNetworkListMsgData contains a list of mana values for nodes in the network.
type ManaNetworkListMsgData struct {
	ManaType  string         `json:"manaType"`
	TotalMana float64        `json:"totalMana"`
	Nodes     []mana.NodeStr `json:"nodes"`
}

// AllowedPledgeIDsMsgData contains information on the allowed pledge ID configuration of the node.
type AllowedPledgeIDsMsgData struct {
	Access    PledgeIDFilter `json:"accessFilter"`
	Consensus PledgeIDFilter `json:"consensusFilter"`
}

// PledgeIDFilter defines if the filter is enabled, and what nodeIDs are allowed.
type PledgeIDFilter struct {
	Enabled        bool             `json:"enabled"`
	AllowedNodeIDs []AllowedNodeStr `json:"allowedNodeIDs"`
}

// AllowedNodeStr contains the short and full nodeIDs of a node.
type AllowedNodeStr struct {
	ShortID string `json:"shortID"`
	FullID  string `json:"fullID"`
}

// endregion
