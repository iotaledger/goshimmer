package request

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/pkg/errors"
    "net"
)

var OUTGOING_REQUEST *Request

func Send(peer *peer.Peer) (bool, error) {
    var keepAlive bool
    switch peer.PeeringProtocolType {
    case types.PROTOCOL_TYPE_TCP:
        keepAlive = true
    case types.PROTOCOL_TYPE_UDP:
        keepAlive = false
    default:
        return false, errors.New("peer uses invalid protocol")
    }

    if err := peer.Send(OUTGOING_REQUEST.Marshal(), keepAlive); err != nil {
        return false, err
    }

    return keepAlive, nil
}

func init() {
    OUTGOING_REQUEST = &Request{
        Issuer: &peer.Peer{
            Identity:            accountability.OWN_ID,
            PeeringProtocolType: types.PROTOCOL_TYPE_TCP,
            PeeringPort:         uint16(*parameters.UDP_PORT.Value),
            GossipProtocolType:  types.PROTOCOL_TYPE_TCP,
            GossipPort:          uint16(*parameters.UDP_PORT.Value),
            Address:             net.IPv4(0, 0, 0, 0),
            Salt:                saltmanager.PUBLIC_SALT,
        },
    }
    OUTGOING_REQUEST.Sign()

    saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
        OUTGOING_REQUEST.Sign()
    })
}
