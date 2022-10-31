package p2p

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/libp2putil/libp2ptesting"
	p2pproto "github.com/iotaledger/goshimmer/packages/network/p2p/proto"
)

var (
	testPacket1             = &p2pproto.Negotiation{}
	log                     = logger.NewExampleLogger("p2p_test")
	protocolID  protocol.ID = "testgossip/0.0.1"
)

func TestNeighborClose(t *testing.T) {
	a, _, teardown := libp2ptesting.NewStreamsPipe(t)
	defer teardown()

	n := newTestNeighbor("A", a)
	n.readLoop()
	require.NoError(t, n.disconnect())
}

func TestNeighborCloseTwice(t *testing.T) {
	a, _, teardown := libp2ptesting.NewStreamsPipe(t)
	defer teardown()

	n := newTestNeighbor("A", a)
	n.readLoop()
	require.NoError(t, n.disconnect())
	require.NoError(t, n.disconnect())
}

func TestNeighborWrite(t *testing.T) {
	a, b, teardown := libp2ptesting.NewStreamsPipe(t)
	defer teardown()

	neighborA := newTestNeighbor("A", a)
	defer neighborA.disconnect()
	var countA uint32
	neighborA.Events.PacketReceived.Hook(event.NewClosure(func(event *NeighborPacketReceivedEvent) {
		_ = event.Packet.(*p2pproto.Negotiation)
		atomic.AddUint32(&countA, 1)
	}))
	neighborA.readLoop()

	neighborB := newTestNeighbor("B", b)
	defer neighborB.disconnect()

	var countB uint32
	neighborB.Events.PacketReceived.Hook(event.NewClosure(func(event *NeighborPacketReceivedEvent) {
		_ = event.Packet.(*p2pproto.Negotiation)
		atomic.AddUint32(&countB, 1)
	}))
	neighborB.readLoop()

	err := neighborA.protocols[protocolID].WritePacket(testPacket1)
	require.NoError(t, err)
	err = neighborB.protocols[protocolID].WritePacket(testPacket1)
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return atomic.LoadUint32(&countA) == 1 }, time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return atomic.LoadUint32(&countB) == 1 }, time.Second, 10*time.Millisecond)
}

func newTestNeighbor(name string, stream network.Stream) *Neighbor {
	return NewNeighbor(newTestPeer(name), NeighborsGroupAuto, map[protocol.ID]*PacketsStream{protocolID: NewPacketsStream(stream, packetFactory)}, log.Named(name))
}

func packetFactory() proto.Message {
	return &p2pproto.Negotiation{}
}

func newTestPeer(name string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, "tcp", 0)
	services.Update(service.P2PKey, "tcp", 0)

	var publicKey ed25519.PublicKey
	copy(publicKey[:], name)

	return peer.NewPeer(identity.New(publicKey), net.IPv4zero, services)
}
