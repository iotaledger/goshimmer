package dashboard

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/core/mana"

	"github.com/iotaledger/goshimmer/packages/node/shutdown"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/blocklayer"
)

var (
	manaFeedWorkerCount     = 1
	manaFeedWorkerQueueSize = 500
	manaFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	manaBuffer              *ManaBuffer
	manaBufferOnce          sync.Once
)

// ManaBufferInstance is the ManaBuffer singleton.
func ManaBufferInstance() *ManaBuffer {
	manaBufferOnce.Do(func() {
		manaBuffer = NewManaBuffer()
	})
	return manaBuffer
}

func configureManaFeed() {
	manaFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
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
	notifyManaPledge := event.NewClosure(func(ev *mana.PledgedEvent) {
		manaFeedWorkerPool.TrySubmit(MsgTypeManaPledge, ev)
	})
	notifyManaRevoke := event.NewClosure(func(ev *mana.RevokedEvent) {
		manaFeedWorkerPool.TrySubmit(MsgTypeManaRevoke, ev)
	})
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(ctx context.Context) {
		mana.Events.Pledged.Attach(notifyManaPledge)
		mana.Events.Revoked.Attach(notifyManaRevoke)
		manaTicker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ctx.Done():
				log.Info("Stopping Dashboard[ManaUpdater] ...")
				manaFeedWorkerPool.Stop()
				manaTicker.Stop()
				log.Info("Stopping Dashboard[ManaUpdater] ... done")
				return
			case <-manaTicker.C:
				manaFeedWorkerPool.TrySubmit(MsgTypeManaValue)
				manaFeedWorkerPool.TrySubmit(MsgTypeManaMapOverall)
				manaFeedWorkerPool.TrySubmit(MsgTypeManaMapOnline)
			}
		}
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// region Websocket block sending handlers (live updates)
func sendManaValue() {
	ownID := deps.Local.ID()
	access, _, err := manaPlugin.GetAccessMana(ownID)
	// if node not found, returned value is 0.0
	if err != nil && !errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) && !errors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get own access mana: %s ", err.Error())
	}
	consensus, _, err := manaPlugin.GetConsensusMana(ownID)
	// if node not found, returned value is 0.0
	if err != nil && !errors.Is(err, mana.ErrNodeNotFoundInBaseManaVector) && !errors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get own consensus mana: %s ", err.Error())
	}
	blkData := &ManaValueBlkData{
		NodeID:    ownID.String(),
		Access:    access,
		Consensus: consensus,
		Time:      time.Now().Unix(),
	}
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaValue,
		Data: blkData,
	})
	ManaBufferInstance().StoreValueBlk(blkData)
}

func sendManaMapOverall() {
	accessManaList, _, err := manaPlugin.GetHighestManaNodes(mana.AccessMana, 0)
	if err != nil && !errors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest access mana nodes: %s ", err.Error())
	}
	accessPayload := &ManaNetworkListBlkData{ManaType: mana.AccessMana.String()}
	totalAccessMana := 0.0
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOverall,
		Data: accessPayload,
	})
	consensusManaList, _, err := manaPlugin.GetHighestManaNodes(mana.ConsensusMana, 0)
	if err != nil && !errors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest consensus mana nodes: %s ", err.Error())
	}
	consensusPayload := &ManaNetworkListBlkData{ManaType: mana.ConsensusMana.String()}
	totalConsensusMana := 0.0
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Nodes = append(consensusPayload.Nodes, consensusManaList[i].ToNodeStr())
		totalConsensusMana += consensusManaList[i].Mana
	}
	consensusPayload.TotalMana = totalConsensusMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOverall,
		Data: consensusPayload,
	})
	ManaBufferInstance().StoreMapOverall(accessPayload, consensusPayload)
}

func sendManaMapOnline() {
	accessManaList, _, err := manaPlugin.GetOnlineNodes(mana.AccessMana)
	if err != nil && !errors.Is(err, manaPlugin.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of online access mana nodes: %s", err.Error())
	}
	accessPayload := &ManaNetworkListBlkData{ManaType: mana.AccessMana.String()}
	totalAccessMana := 0.0
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Nodes = append(accessPayload.Nodes, accessManaList[i].ToNodeStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOnline,
		Data: accessPayload,
	})

	weights, totalWeight := deps.Tangle.WeightProvider.WeightsOfRelevantVoters()
	consensusPayload := &ManaNetworkListBlkData{ManaType: mana.ConsensusMana.String()}
	for nodeID, weight := range weights {
		n := mana.Node{
			ID:   nodeID,
			Mana: weight,
		}
		consensusPayload.Nodes = append(consensusPayload.Nodes, n.ToNodeStr())
	}

	sort.Slice(consensusPayload.Nodes, func(i, j int) bool {
		return consensusPayload.Nodes[i].Mana > consensusPayload.Nodes[j].Mana
	})

	consensusPayload.TotalMana = totalWeight
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOnline,
		Data: consensusPayload,
	})
	ManaBufferInstance().StoreMapOnline(accessPayload, consensusPayload)
}

func sendManaPledge(ev *mana.PledgedEvent) {
	ManaBufferInstance().StoreEvent(ev)
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaPledge,
		Data: ev.ToJSONSerializable(),
	})
}

func sendManaRevoke(ev *mana.RevokedEvent) {
	ManaBufferInstance().StoreEvent(ev)
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaRevoke,
		Data: ev.ToJSONSerializable(),
	})
}

// endregion

// region Websocket block sending handlers (initial data)
func sendAllowedManaPledge(ws *websocket.Conn) error {
	allowedAccess := manaPlugin.GetAllowedPledgeNodes(mana.AccessMana)
	allowedConsensus := manaPlugin.GetAllowedPledgeNodes(mana.ConsensusMana)

	wsblkData := &AllowedPledgeIDsBlkData{}
	wsblkData.Access.Enabled = allowedAccess.IsFilterEnabled
	allowedAccess.Allowed.ForEach(func(ID identity.ID) {
		wsblkData.Access.AllowedNodeIDs = append(wsblkData.Access.AllowedNodeIDs, AllowedNodeStr{
			ShortID: ID.String(),
			FullID:  base58.Encode(ID.Bytes()),
		})
	})
	wsblkData.Consensus.Enabled = allowedConsensus.IsFilterEnabled
	allowedConsensus.Allowed.ForEach(func(ID identity.ID) {
		wsblkData.Consensus.AllowedNodeIDs = append(wsblkData.Consensus.AllowedNodeIDs, AllowedNodeStr{
			ShortID: ID.String(),
			FullID:  base58.Encode(ID.Bytes()),
		})
	})

	if err := sendJSON(ws, &wsblk{
		Type: MsgTypeManaAllowedPledge,
		Data: wsblkData,
	}); err != nil {
		return err
	}
	return nil
}

// endregion

// region Websocket block data structs

// ManaValueBlkData contains mana values for a particular node.
type ManaValueBlkData struct {
	NodeID    string  `json:"nodeID"`
	Access    float64 `json:"access"`
	Consensus float64 `json:"consensus"`
	Time      int64   `json:"time"`
}

// ManaNetworkListBlkData contains a list of mana values for nodes in the network.
type ManaNetworkListBlkData struct {
	ManaType  string         `json:"manaType"`
	TotalMana float64        `json:"totalMana"`
	Nodes     []mana.NodeStr `json:"nodes"`
}

// AllowedPledgeIDsBlkData contains information on the allowed pledge ID configuration of the node.
type AllowedPledgeIDsBlkData struct {
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
