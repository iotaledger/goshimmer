package client

import (
	"io"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	manaPlugin "github.com/iotaledger/goshimmer/plugins/mana"
)

func createManaHeartbeat() (*packet.ManaHeartbeat, error) {
	all := manaPlugin.GetAllManaMaps(mana.Mixed)
	onlineAccess, err := manaPlugin.GetOnlineNodes(mana.AccessMana, mana.Mixed)
	if err != nil {
		return nil, err
	}
	onlineConsensus, err := manaPlugin.GetOnlineNodes(mana.ConsensusMana, mana.Mixed)
	if err != nil {
		return nil, err
	}
	online := map[mana.Type][]mana.Node{
		mana.AccessMana:    onlineAccess,
		mana.ConsensusMana: onlineConsensus,
	}
	return &packet.ManaHeartbeat{
		Version:          banner.AppVersion,
		NetworkMap:       all,
		OnlineNetworkMap: online,
		PledgeEvents:     GetPledgeEvents(),
		RevokeEvents:     GetRevokeEvents(),
		NodeID:           local.GetInstance().ID(),
	}, nil
}

func sendManaHeartbeat(w io.Writer, hb *packet.ManaHeartbeat) {
	data, err := packet.NewManaHeartbeatMessage(hb)
	if err != nil {
		log.Debugw("mana heartbeat message skipped", "err", err)
		return
	}

	if _, err = w.Write(data); err != nil {
		log.Debugw("Error while writing to connection", "Description", err)
	}
	// trigger AnalysisOutboundBytes event
	metrics.Events().AnalysisOutboundBytes.Trigger(uint64(len(data)))
}
