package peer

import (
	"encoding/binary"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/identity"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/salt"
	"github.com/pkg/errors"
)

type Peer struct {
	identity         *identity.Identity
	identityMutex    sync.RWMutex
	address          net.IP
	addressMutex     sync.RWMutex
	peeringPort      uint16
	peeringPortMutex sync.RWMutex
	gossipPort       uint16
	gossipPortMutex  sync.RWMutex
	salt             *salt.Salt
	saltMutex        sync.RWMutex
	conn             *network.ManagedConnection
	connectMutex     sync.RWMutex
	firstSeen        time.Time
	firstSeenMutex   sync.RWMutex
	lastSeen         time.Time
	lastSeenMutex    sync.RWMutex
}

func (peer *Peer) GetIdentity() (result *identity.Identity) {
	peer.identityMutex.RLock()
	result = peer.identity
	peer.identityMutex.RUnlock()

	return
}

func (peer *Peer) SetIdentity(identity *identity.Identity) {
	peer.identityMutex.Lock()
	peer.identity = identity
	peer.identityMutex.Unlock()
}

func (peer *Peer) GetAddress() (result net.IP) {
	peer.addressMutex.RLock()
	result = peer.address
	peer.addressMutex.RUnlock()

	return
}

func (peer *Peer) SetAddress(address net.IP) {
	peer.addressMutex.Lock()
	peer.address = address
	peer.addressMutex.Unlock()
}

func (peer *Peer) GetPeeringPort() (result uint16) {
	peer.peeringPortMutex.RLock()
	result = peer.peeringPort
	peer.peeringPortMutex.RUnlock()

	return
}

func (peer *Peer) SetPeeringPort(port uint16) {
	peer.peeringPortMutex.Lock()
	peer.peeringPort = port
	peer.peeringPortMutex.Unlock()
}

func (peer *Peer) GetGossipPort() (result uint16) {
	peer.gossipPortMutex.RLock()
	result = peer.gossipPort
	peer.gossipPortMutex.RUnlock()

	return
}

func (peer *Peer) SetGossipPort(port uint16) {
	peer.gossipPortMutex.Lock()
	peer.gossipPort = port
	peer.gossipPortMutex.Unlock()
}

func (peer *Peer) GetSalt() (result *salt.Salt) {
	peer.saltMutex.RLock()
	result = peer.salt
	peer.saltMutex.RUnlock()

	return
}

func (peer *Peer) SetSalt(salt *salt.Salt) {
	peer.saltMutex.Lock()
	peer.salt = salt
	peer.saltMutex.Unlock()
}

func (peer *Peer) GetConn() (result *network.ManagedConnection) {
	peer.connectMutex.RLock()
	result = peer.conn
	peer.connectMutex.RUnlock()

	return
}

func (peer *Peer) SetConn(conn *network.ManagedConnection) {
	peer.connectMutex.Lock()
	peer.conn = conn
	peer.connectMutex.Unlock()
}

func Unmarshal(data []byte) (*Peer, error) {
	if len(data) < MARSHALED_TOTAL_SIZE {
		return nil, errors.New("size of marshaled peer is too small")
	}

	peer := &Peer{
		identity: identity.NewIdentity(data[MARSHALED_PUBLIC_KEY_START:MARSHALED_PUBLIC_KEY_END]),
	}

	switch data[MARSHALED_ADDRESS_TYPE_START] {
	case types.ADDRESS_TYPE_IPV4:
		peer.address = net.IP(data[MARSHALED_ADDRESS_START:MARSHALED_ADDRESS_END]).To4()
	case types.ADDRESS_TYPE_IPV6:
		peer.address = net.IP(data[MARSHALED_ADDRESS_START:MARSHALED_ADDRESS_END]).To16()
	}

	peer.peeringPort = binary.BigEndian.Uint16(data[MARSHALED_PEERING_PORT_START:MARSHALED_PEERING_PORT_END])
	peer.gossipPort = binary.BigEndian.Uint16(data[MARSHALED_GOSSIP_PORT_START:MARSHALED_GOSSIP_PORT_END])

	if unmarshaledSalt, err := salt.Unmarshal(data[MARSHALED_SALT_START:MARSHALED_SALT_END]); err != nil {
		return nil, err
	} else {
		peer.salt = unmarshaledSalt
	}

	return peer, nil
}

// sends data and
func (peer *Peer) Send(data []byte, protocol types.ProtocolType, responseExpected bool) (bool, error) {
	conn, dialed, err := peer.Connect(protocol)
	if err != nil {
		return false, err
	}

	if _, err := conn.Write(data); err != nil {
		return false, err
	}

	if dialed && !responseExpected {
		conn.Close()
	}

	return dialed, nil
}

func (peer *Peer) ConnectTCP() (*network.ManagedConnection, bool, error) {
	peer.connectMutex.RLock()

	if peer.conn == nil {
		peer.connectMutex.RUnlock()
		peer.connectMutex.Lock()
		defer peer.connectMutex.Unlock()

		if peer.conn == nil {
			conn, err := net.Dial("tcp", peer.GetAddress().String()+":"+strconv.Itoa(int(peer.GetPeeringPort())))
			if err != nil {
				return nil, false, errors.New("error when connecting to " + peer.String() + ": " + err.Error())
			} else {
				peer.conn = network.NewManagedConnection(conn)
				peer.conn.Events.Close.Attach(events.NewClosure(func() {
					peer.SetConn(nil)
				}))

				return peer.conn, true, nil
			}
		}
	} else {
		peer.connectMutex.RUnlock()
	}

	return peer.conn, false, nil
}

func (peer *Peer) ConnectUDP() (*network.ManagedConnection, bool, error) {
	conn, err := net.Dial("udp", peer.GetAddress().String()+":"+strconv.Itoa(int(peer.GetPeeringPort())))
	if err != nil {
		return nil, false, errors.New("error when connecting to " + peer.GetAddress().String() + ": " + err.Error())
	}

	return network.NewManagedConnection(conn), true, nil
}

func (peer *Peer) Connect(protocol types.ProtocolType) (*network.ManagedConnection, bool, error) {
	switch protocol {
	case types.PROTOCOL_TYPE_TCP:
		return peer.ConnectTCP()
	case types.PROTOCOL_TYPE_UDP:
		return peer.ConnectUDP()
	default:
		return nil, false, errors.New("unsupported peering protocol in peer " + peer.GetAddress().String())
	}
}

func (peer *Peer) Marshal() []byte {
	result := make([]byte, MARSHALED_TOTAL_SIZE)

	copy(result[MARSHALED_PUBLIC_KEY_START:MARSHALED_PUBLIC_KEY_END],
		peer.GetIdentity().PublicKey[:MARSHALED_PUBLIC_KEY_SIZE])

	switch len(peer.GetAddress()) {
	case net.IPv4len:
		result[MARSHALED_ADDRESS_TYPE_START] = types.ADDRESS_TYPE_IPV4
	case net.IPv6len:
		result[MARSHALED_ADDRESS_TYPE_START] = types.ADDRESS_TYPE_IPV6
	default:
		panic("invalid address in peer")
	}

	copy(result[MARSHALED_ADDRESS_START:MARSHALED_ADDRESS_END], peer.GetAddress().To16())

	binary.BigEndian.PutUint16(result[MARSHALED_PEERING_PORT_START:MARSHALED_PEERING_PORT_END], peer.GetPeeringPort())
	binary.BigEndian.PutUint16(result[MARSHALED_GOSSIP_PORT_START:MARSHALED_GOSSIP_PORT_END], peer.GetGossipPort())

	copy(result[MARSHALED_SALT_START:MARSHALED_SALT_END], peer.GetSalt().Marshal())

	return result
}

func (peer *Peer) String() string {
	if peer.GetIdentity() != nil {
		return peer.GetAddress().String() + ":" + strconv.Itoa(int(peer.GetPeeringPort())) + " / " + peer.GetIdentity().StringIdentifier
	} else {
		return peer.GetAddress().String() + ":" + strconv.Itoa(int(peer.GetPeeringPort()))
	}
}
