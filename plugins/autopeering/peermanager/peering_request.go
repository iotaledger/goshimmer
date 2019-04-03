package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "net"
)

var PEERING_REQUEST *request.Request

func configurePeeringRequest() {
    PEERING_REQUEST = &request.Request{
        Issuer: &peer.Peer{
            Identity:            accountability.OWN_ID,
            PeeringProtocolType: protocol.PROTOCOL_TYPE_TCP,
            PeeringPort:         uint16(*parameters.UDP_PORT.Value),
            GossipProtocolType:  protocol.PROTOCOL_TYPE_TCP,
            GossipPort:          uint16(*parameters.UDP_PORT.Value),
            Address:             net.IPv4(0, 0, 0, 0),
        },
        Salt: saltmanager.PUBLIC_SALT,
    }
    PEERING_REQUEST.Sign()

    saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
        PEERING_REQUEST.Sign()
    })
}
