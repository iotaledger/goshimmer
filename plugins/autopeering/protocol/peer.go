package protocol

import (
    "encoding/binary"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/pkg/errors"
    "net"
    "time"
)

type Peer struct {
    Identity            *identity.Identity
    Address             net.IP
    PeeringProtocolType ProtocolType
    PeeringPort         uint16
    GossipProtocolType  ProtocolType
    GossipPort          uint16
    FirstSeen           time.Time
    LastSeen            time.Time
    LastContact         time.Time
}

func UnmarshalPeer(data []byte) (*Peer, error) {
    if len(data) < PEER_MARSHALLED_TOTAL_SIZE {
        return nil, errors.New("size of marshalled peer is too small")
    }

    peer := &Peer{
        Identity: identity.NewIdentity(data[PEER_MARSHALLED_PUBLIC_KEY_START:PEER_MARSHALLED_PUBLIC_KEY_END]),
    }

    switch data[PEER_MARSHALLED_ADDRESS_TYPE_START] {
    case PEER_MARSHALLED_ADDRESS_TYPE_IPV4:
        peer.Address = net.IP(data[PEER_MARSHALLED_ADDRESS_START:PEER_MARSHALLED_ADDRESS_END]).To4()
    case PEER_MARSHALLED_ADDRESS_TYPE_IPV6:
        peer.Address = net.IP(data[PEER_MARSHALLED_ADDRESS_START:PEER_MARSHALLED_ADDRESS_END]).To16()
    }

    peer.PeeringProtocolType = ProtocolType(data[PEER_MARSHALLED_PEERING_PROTOCOL_TYPE_START])
    peer.PeeringPort = binary.BigEndian.Uint16(data[PEER_MARSHALLED_PEERING_PORT_START:PEER_MARSHALLED_PEERING_PORT_END])

    peer.GossipProtocolType = ProtocolType(data[PEER_MARSHALLED_GOSSIP_PROTOCOL_TYPE_START])
    peer.GossipPort = binary.BigEndian.Uint16(data[PEER_MARSHALLED_GOSSIP_PORT_START:PEER_MARSHALLED_GOSSIP_PORT_END])

    return peer, nil
}

func (peer *Peer) Marshal() []byte {
    result := make([]byte, PEER_MARSHALLED_TOTAL_SIZE)

    copy(result[PEER_MARSHALLED_PUBLIC_KEY_START:PEER_MARSHALLED_PUBLIC_KEY_END],
        peer.Identity.PublicKey[:PEER_MARSHALLED_PUBLIC_KEY_SIZE])

    switch len(peer.Address) {
    case net.IPv4len:
        result[PEER_MARSHALLED_ADDRESS_TYPE_START] = PEER_MARSHALLED_ADDRESS_TYPE_IPV4
    case net.IPv6len:
        result[PEER_MARSHALLED_ADDRESS_TYPE_START] = PEER_MARSHALLED_ADDRESS_TYPE_IPV6
    default:
        panic("invalid address in peer")
    }

    copy(result[PEER_MARSHALLED_ADDRESS_START:PEER_MARSHALLED_ADDRESS_END], peer.Address.To16())

    result[PEER_MARSHALLED_PEERING_PROTOCOL_TYPE_START] = peer.PeeringProtocolType
    binary.BigEndian.PutUint16(result[PEER_MARSHALLED_PEERING_PORT_START:PEER_MARSHALLED_PEERING_PORT_END], peer.PeeringPort)

    result[PEER_MARSHALLED_GOSSIP_PROTOCOL_TYPE_START] = peer.GossipProtocolType
    binary.BigEndian.PutUint16(result[PEER_MARSHALLED_GOSSIP_PORT_START:PEER_MARSHALLED_GOSSIP_PORT_END], peer.GossipPort)

    return result
}
