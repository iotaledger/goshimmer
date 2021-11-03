package gossip

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/libp2putil/libp2ptesting"
)

var testPacket = &pb.Packet{Body: &pb.Packet_Message{Message: &pb.Message{Data: []byte("foobar")}}}

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
	neighborA.readLoop()

	neighborB := newTestNeighbor("B", b)
	defer neighborB.disconnect()

	var count uint32
	neighborB.packetReceived.Attach(events.NewClosure(func(packet *pb.Packet) {
		assert.Equal(t, testPacket.String(), packet.String())
		atomic.AddUint32(&count, 1)
	}))
	neighborB.readLoop()

	err := neighborA.write(testPacket)
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return atomic.LoadUint32(&count) == 1 }, time.Second, 10*time.Millisecond)
}

func newTestNeighbor(name string, stream network.Stream) *Neighbor {
	return NewNeighbor(newTestPeer(name), NeighborsGroupAuto, stream, log.Named(name))
}

func newTestPeer(name string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, "tcp", 0)
	services.Update(service.GossipKey, "tcp", 0)

	var publicKey ed25519.PublicKey
	copy(publicKey[:], name)

	return peer.NewPeer(identity.New(publicKey), net.IPv4zero, services)
}
