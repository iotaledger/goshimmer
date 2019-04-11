package peer

import (
    "encoding/binary"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "github.com/pkg/errors"
    "net"
    "strconv"
    "sync"
)

type Peer struct {
    Identity            *identity.Identity
    Address             net.IP
    PeeringProtocolType types.ProtocolType
    PeeringPort         uint16
    GossipProtocolType  types.ProtocolType
    GossipPort          uint16
    Salt                *salt.Salt
    Conn                *network.ManagedConnection
    connectMutex        sync.Mutex
}

func Unmarshal(data []byte) (*Peer, error) {
    if len(data) < MARSHALLED_TOTAL_SIZE {
        return nil, errors.New("size of marshalled peer is too small")
    }

    peer := &Peer{
        Identity: identity.NewIdentity(data[MARSHALLED_PUBLIC_KEY_START:MARSHALLED_PUBLIC_KEY_END]),
    }

    switch data[MARSHALLED_ADDRESS_TYPE_START] {
    case types.ADDRESS_TYPE_IPV4:
        peer.Address = net.IP(data[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END]).To4()
    case types.ADDRESS_TYPE_IPV6:
        peer.Address = net.IP(data[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END]).To16()
    }

    peer.PeeringProtocolType = types.ProtocolType(data[MARSHALLED_PEERING_PROTOCOL_TYPE_START])
    peer.PeeringPort = binary.BigEndian.Uint16(data[MARSHALLED_PEERING_PORT_START:MARSHALLED_PEERING_PORT_END])

    peer.GossipProtocolType = types.ProtocolType(data[MARSHALLED_GOSSIP_PROTOCOL_TYPE_START])
    peer.GossipPort = binary.BigEndian.Uint16(data[MARSHALLED_GOSSIP_PORT_START:MARSHALLED_GOSSIP_PORT_END])

    if unmarshalledSalt, err := salt.Unmarshal(data[MARSHALLED_SALT_START:MARSHALLED_SALT_END]); err != nil {
        return nil, err
    } else {
        peer.Salt = unmarshalledSalt
    }

    return peer, nil
}

func (peer *Peer) Send(data []byte, keepConnectionAlive bool) error {
    newConnection, err := peer.Connect()
    if err != nil {
        return err
    }

    if _, err := peer.Conn.Write(data); err != nil {
        return err
    }

    if newConnection && !keepConnectionAlive {
        peer.Conn.Close()
    }

    return nil
}

func (peer *Peer) Connect() (bool, error) {
    if peer.Conn == nil {
        peer.connectMutex.Lock()
        defer peer.connectMutex.Unlock()

        if peer.Conn == nil {
            var protocolString string
            switch peer.PeeringProtocolType {
            case types.PROTOCOL_TYPE_TCP:
                protocolString = "tcp"
            case types.PROTOCOL_TYPE_UDP:
                protocolString = "udp"
            default:
                return false, errors.New("unsupported peering protocol in peer " + peer.Address.String())
            }

            conn, err := net.Dial(protocolString, peer.Address.String()+":"+strconv.Itoa(int(peer.PeeringPort)))
            if err != nil {
                return false, errors.New("error when connecting to " + peer.Address.String() + " during peering process: " + err.Error())
            } else {
                peer.Conn = network.NewManagedConnection(conn)

                peer.Conn.Events.Close.Attach(func() {
                    peer.Conn = nil
                })

                return true, nil
            }
        }
    }

    return false, nil
}

func (peer *Peer) Marshal() []byte {
    result := make([]byte, MARSHALLED_TOTAL_SIZE)

    copy(result[MARSHALLED_PUBLIC_KEY_START:MARSHALLED_PUBLIC_KEY_END],
        peer.Identity.PublicKey[:MARSHALLED_PUBLIC_KEY_SIZE])

    switch len(peer.Address) {
    case net.IPv4len:
        result[MARSHALLED_ADDRESS_TYPE_START] = types.ADDRESS_TYPE_IPV4
    case net.IPv6len:
        result[MARSHALLED_ADDRESS_TYPE_START] = types.ADDRESS_TYPE_IPV6
    default:
        panic("invalid address in peer")
    }

    copy(result[MARSHALLED_ADDRESS_START:MARSHALLED_ADDRESS_END], peer.Address.To16())

    result[MARSHALLED_PEERING_PROTOCOL_TYPE_START] = peer.PeeringProtocolType
    binary.BigEndian.PutUint16(result[MARSHALLED_PEERING_PORT_START:MARSHALLED_PEERING_PORT_END], peer.PeeringPort)

    result[MARSHALLED_GOSSIP_PROTOCOL_TYPE_START] = peer.GossipProtocolType
    binary.BigEndian.PutUint16(result[MARSHALLED_GOSSIP_PORT_START:MARSHALLED_GOSSIP_PORT_END], peer.GossipPort)

    copy(result[MARSHALLED_SALT_START:MARSHALLED_SALT_END], peer.Salt.Marshal())

    return result
}

func (peer *Peer) String() string {
    if peer.Identity != nil {
        return peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)) + " / " + peer.Identity.StringIdentifier
    } else {
        return peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort))
    }
}
