package dashboard

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

var (
	manaBuffer     *ManaBuffer
	manaBufferOnce sync.Once
)

// ManaBufferInstance is the ManaBuffer singleton.
func ManaBufferInstance() *ManaBuffer {
	manaBufferOnce.Do(func() {
		manaBuffer = NewManaBuffer()
	})
	return manaBuffer
}

func runManaFeed(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(ctx context.Context) {
		manaTicker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ctx.Done():
				log.Info("Stopping Dashboard[ManaUpdater] ...")
				manaTicker.Stop()
				log.Info("Stopping Dashboard[ManaUpdater] ... done")
				return
			case <-manaTicker.C:
				plugin.WorkerPool.Submit(sendManaValue)
				plugin.WorkerPool.Submit(sendManaMapOverall)
				plugin.WorkerPool.Submit(sendManaMapOnline)
			}
		}
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// region Websocket block sending handlers (live updates).
func sendManaValue() {
	ownID := deps.Local.ID()
	access, exists := deps.Protocol.Engine().ThroughputQuota.Balance(ownID)
	// if issuer not found, returned value is 0.0
	if !exists {
		log.Debugf("no mana available for local identity: %s ", ownID.String())
	}

	ownWeight, exists := deps.Protocol.Engine().SybilProtection.Weights().Get(ownID)
	if !exists {
		ownWeight = sybilprotection.NewWeight(0, -1)
	}

	blkData := &ManaValueBlkData{
		IssuerID:  ownID.String(),
		Access:    access,
		Consensus: ownWeight.Value,
		Time:      time.Now().Unix(),
	}

	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaValue,
		Data: blkData,
	})
	ManaBufferInstance().StoreValueBlk(blkData)
}

func sendManaMapOverall() {
	accessManaList, _, err := manamodels.GetHighestManaIssuers(0, deps.Protocol.Engine().ThroughputQuota.BalanceByIDs())
	if err != nil && !errors.Is(err, manamodels.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest access mana issuers: %s ", err.Error())
	}
	accessPayload := &ManaNetworkListBlkData{ManaType: manamodels.AccessMana.String()}
	totalAccessMana := int64(0)
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Issuers = append(accessPayload.Issuers, accessManaList[i].ToIssuerStr())
		totalAccessMana += accessManaList[i].Mana
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOverall,
		Data: accessPayload,
	})
	consensusManaList, _, err := manamodels.GetHighestManaIssuers(0, lo.PanicOnErr(deps.Protocol.Engine().SybilProtection.Weights().Map()))
	if err != nil && !errors.Is(err, manamodels.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest consensus mana issuers: %s ", err.Error())
	}
	consensusPayload := &ManaNetworkListBlkData{ManaType: manamodels.ConsensusMana.String()}

	var totalConsensusMana int64
	for i := 0; i < len(consensusManaList); i++ {
		consensusPayload.Issuers = append(consensusPayload.Issuers, consensusManaList[i].ToIssuerStr())
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
	if deps.Discover == nil {
		return
	}
	knownPeers := deps.Discover.GetVerifiedPeers()
	manaMap := deps.Protocol.Engine().ThroughputQuota.BalanceByIDs()
	accessPayload := &ManaNetworkListBlkData{ManaType: manamodels.AccessMana.String()}
	var totalAccessMana int64
	for _, peerID := range append(lo.Map(knownPeers, func(p *peer.Peer) identity.ID { return p.ID() }), deps.Local.ID()) {
		manaValue, exists := manaMap[peerID]
		if !exists {
			continue
		}

		accessPayload.Issuers = append(accessPayload.Issuers, manamodels.IssuerStr{
			ShortIssuerID: peerID.String(),
			IssuerID:      base58.Encode(lo.PanicOnErr(peerID.Bytes())),
			Mana:          manaValue,
		})
		totalAccessMana += manaValue
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOnline,
		Data: accessPayload,
	})

	activeNodes := deps.Protocol.Engine().SybilProtection.Validators()
	consensusPayload := &ManaNetworkListBlkData{ManaType: manamodels.ConsensusMana.String()}

	_ = activeNodes.ForEach(func(id identity.ID) error {
		weight, exists := deps.Protocol.Engine().SybilProtection.Weights().Get(id)
		if !exists {
			weight = sybilprotection.NewWeight(0, -1)
		}
		consensusPayload.Issuers = append(consensusPayload.Issuers, manamodels.Issuer{
			ID:   id,
			Mana: weight.Value,
		}.ToIssuerStr())

		return nil
	})

	sort.Slice(consensusPayload.Issuers, func(i, j int) bool {
		return consensusPayload.Issuers[i].Mana > consensusPayload.Issuers[j].Mana || (consensusPayload.Issuers[i].Mana == consensusPayload.Issuers[j].Mana && consensusPayload.Issuers[i].IssuerID > consensusPayload.Issuers[j].IssuerID)
	})

	consensusPayload.TotalMana = activeNodes.TotalWeight()
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOnline,
		Data: consensusPayload,
	})
	ManaBufferInstance().StoreMapOnline(accessPayload, consensusPayload)
}

// endregion

// region Websocket block data structs

// ManaValueBlkData contains mana values for a particular issuer.
type ManaValueBlkData struct {
	IssuerID  string `json:"nodeID"`
	Access    int64  `json:"access"`
	Consensus int64  `json:"consensus"`
	Time      int64  `json:"time"`
}

// ManaNetworkListBlkData contains a list of mana values for issuers in the network.
type ManaNetworkListBlkData struct {
	ManaType  string                 `json:"manaType"`
	TotalMana int64                  `json:"totalMana"`
	Issuers   []manamodels.IssuerStr `json:"nodes"`
}

// endregion
