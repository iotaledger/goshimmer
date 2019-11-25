package ownpeer

import (
	"net"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/node"
	autopeering_params "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/parameter"
)

var INSTANCE *peer.Peer

func Configure(plugin *node.Plugin) {
	INSTANCE = &peer.Peer{}
	INSTANCE.SetIdentity(accountability.OwnId())
	INSTANCE.SetPeeringPort(uint16(parameter.NodeConfig.GetInt(autopeering_params.CFG_PORT)))
	INSTANCE.SetGossipPort(uint16(parameter.NodeConfig.GetInt(gossip.GOSSIP_PORT)))
	INSTANCE.SetAddress(net.IPv4(0, 0, 0, 0))
	INSTANCE.SetSalt(saltmanager.PUBLIC_SALT)
}
