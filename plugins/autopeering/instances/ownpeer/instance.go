package ownpeer

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/gossip"
    "net"
)

var INSTANCE *peer.Peer

func Configure(plugin *node.Plugin) {
    INSTANCE = &peer.Peer{
        Identity:    accountability.OWN_ID,
        PeeringPort: uint16(*parameters.PORT.Value),
        GossipPort:  uint16(*gossip.PORT.Value),
        Address:     net.IPv4(0, 0, 0, 0),
        Salt:        saltmanager.PUBLIC_SALT,
    }
}
