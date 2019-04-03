package peer

import (
    "encoding/binary"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/pkg/errors"
    "net"
    "strconv"
)

type Peer struct {
    Identity            *identity.Identity
    Address             net.IP
    PeeringProtocolType protocol.ProtocolType
    PeeringPort         uint16
    GossipProtocolType  protocol.ProtocolType
    GossipPort          uint16
}

func Unmarshal(data []byte) (*Peer, error) {
    if len(data) < MARSHALLED_TOTAL_SIZE {
        return nil, errors.New("size of marshalled peer is too small")
    }

    peer := &Peer{
        Identity: identity.NewIdentity(data[MARSHALLED_PUBLIC_KEY_START:MARSHALLED_PUBLIC_KEY_END]),
    }

    switch data[MARSHALLED_ADDRESS_TYPE_START] {
    case protocol.ADDRESS_TYPE_IPV4:
        peer.Address = net.IP(data[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END]).To4()
    case protocol.ADDRESS_TYPE_IPV6:
        peer.Address = net.IP(data[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END]).To16()
    }

    peer.PeeringProtocolType = protocol.ProtocolType(data[MARSHALLED_PEERING_PROTOCOL_TYPE_START])
    peer.PeeringPort = binary.BigEndian.Uint16(data[MARSHALLED_PEERING_PORT_START:MARSHALLED_PEERING_PORT_END])

    peer.GossipProtocolType = protocol.ProtocolType(data[MARSHALLED_GOSSIP_PROTOCOL_TYPE_START])
    peer.GossipPort = binary.BigEndian.Uint16(data[MARSHALLED_GOSSIP_PORT_START:MARSHALLED_GOSSIP_PORT_END])

    return peer, nil
}

func (peer *Peer) Marshal() []byte {
    result := make([]byte, MARSHALLED_TOTAL_SIZE)

    copy(result[MARSHALLED_PUBLIC_KEY_START:MARSHALLED_PUBLIC_KEY_END],
        peer.Identity.PublicKey[:MARSHALLED_PUBLIC_KEY_SIZE])

    switch len(peer.Address) {
    case net.IPv4len:
        result[MARSHALLED_ADDRESS_TYPE_START] = protocol.ADDRESS_TYPE_IPV4
    case net.IPv6len:
        result[MARSHALLED_ADDRESS_TYPE_START] = protocol.ADDRESS_TYPE_IPV6
    default:
        panic("invalid address in peer")
    }

    copy(result[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END], peer.Address.To16())

    result[MARSHALLED_PEERING_PROTOCOL_TYPE_START] = peer.PeeringProtocolType
    binary.BigEndian.PutUint16(result[MARSHALLED_PEERING_PORT_START:MARSHALLED_PEERING_PORT_END], peer.PeeringPort)

    result[MARSHALLED_GOSSIP_PROTOCOL_TYPE_START] = peer.GossipProtocolType
    binary.BigEndian.PutUint16(result[MARSHALLED_GOSSIP_PORT_START:MARSHALLED_GOSSIP_PORT_END], peer.GossipPort)

    return result
}

func (peer *Peer) String() string {
    if peer.Identity != nil {
        return peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)) + " / " + peer.Identity.StringIdentifier
    } else {
        return peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort))
    }
}
