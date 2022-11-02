package dashboard

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana/manamodels"
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
		}
		task.Return(nil)
	}, workerpool.WorkerCount(manaFeedWorkerCount), workerpool.QueueSize(manaFeedWorkerQueueSize))
}

func runManaFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ManaUpdater]", func(ctx context.Context) {
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
	access, exists := deps.Protocol.Engine().ManaTracker.Mana(ownID)
	// if issuer not found, returned value is 0.0
	if !exists {
		log.Errorf("failed to get own access mana: %s ", ownID)
	}
	consensus := deps.Protocol.Engine().SybilProtection.Weights()
	blkData := &ManaValueBlkData{
		IssuerID:  ownID.String(),
		Access:    access,
		Consensus: consensus[ownID],
		Time:      time.Now().Unix(),
	}
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaValue,
		Data: blkData,
	})
	ManaBufferInstance().StoreValueBlk(blkData)
}

func sendManaMapOverall() {
	accessManaList, _, err := manamodels.GetHighestManaIssuers(0, deps.Protocol.Engine().ManaTracker.ManaMap())
	if err != nil && !errors.Is(err, manamodels.ErrQueryNotAllowed) {
		log.Errorf("failed to get list of n highest access mana issuers: %s ", err.Error())
	}
	accessPayload := &ManaNetworkListBlkData{ManaType: manamodels.AccessMana.String()}
	totalAccessMana := int64(0)
	for i := 0; i < len(accessManaList); i++ {
		accessPayload.Issuers = append(accessPayload.Issuers, accessManaList[i].ToIssuerStr())
		totalAccessMana += int64(accessManaList[i].Mana)
	}
	accessPayload.TotalMana = totalAccessMana
	broadcastWsBlock(&wsblk{
		Type: MsgTypeManaMapOverall,
		Data: accessPayload,
	})
	consensusManaList, _, err := manamodels.GetHighestManaIssuers(0, deps.Protocol.Engine().SybilProtection.Weights())
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
	manaMap := deps.Protocol.Engine().ManaTracker.ManaMap()
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

	validatorSet := deps.Protocol.Engine().Tangle.ValidatorSet
	consensusPayload := &ManaNetworkListBlkData{ManaType: manamodels.ConsensusMana.String()}
	for _, validator := range validatorSet.Slice() {
		n := manamodels.Issuer{
			ID:   validator.ID(),
			Mana: validator.Weight(),
		}
		consensusPayload.Issuers = append(consensusPayload.Issuers, n.ToIssuerStr())
	}

	sort.Slice(consensusPayload.Issuers, func(i, j int) bool {
		return consensusPayload.Issuers[i].Mana > consensusPayload.Issuers[j].Mana
	})

	consensusPayload.TotalMana = validatorSet.TotalWeight()
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
