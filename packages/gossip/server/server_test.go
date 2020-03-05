package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/database/mapdb"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graceTime = 5 * time.Millisecond

var log = logger.NewExampleLogger("server")

func getPeer(t *TCP) *peer.Peer {
	return &t.local.Peer
}

func TestClose(t *testing.T) {
	_, teardown := newTestServer(t, "A")
	teardown()
}

func TestUnansweredAccept(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()

	_, err := transA.AcceptPeer(getPeer(transA))
	assert.Error(t, err)
}

func TestCloseWhileAccepting(t *testing.T) {
	transA, closeA := newTestServer(t, "A")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := transA.AcceptPeer(getPeer(transA))
		assert.Error(t, err)
	}()
	time.Sleep(graceTime)

	closeA()
	wg.Wait()
}

func TestUnansweredDial(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()

	// create peer with invalid gossip address
	services := getPeer(transA).Services().CreateRecord()
	services.Update(service.GossipKey, "tcp", "localhost:0")
	unreachablePeer := peer.NewPeer(getPeer(transA).PublicKey(), services)

	_, err := transA.DialPeer(unreachablePeer)
	assert.Error(t, err)
}

func TestNoHandshakeResponse(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()

	// accept and read incoming connections
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	go func() {
		conn, _ := lis.Accept()
		n, _ := conn.Read(make([]byte, maxHandshakePacketSize))
		assert.NotZero(t, n)
		_ = conn.Close()
		_ = lis.Close()
	}()

	// create peer for the listener
	services := getPeer(transA).Services().CreateRecord()
	services.Update(service.GossipKey, lis.Addr().Network(), lis.Addr().String())
	p := peer.NewPeer(getPeer(transA).PublicKey(), services)

	_, err = transA.DialPeer(p)
	assert.Error(t, err)
}

func TestNoHandshakeRequest(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := transA.AcceptPeer(getPeer(transA))
		assert.Error(t, err)
	}()
	time.Sleep(graceTime)

	conn, err := net.Dial(transA.LocalAddr().Network(), transA.LocalAddr().String())
	require.NoError(t, err)
	time.Sleep(handshakeTimeout)
	_ = conn.Close()

	wg.Wait()
}

func TestConnect(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()
	transB, closeB := newTestServer(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c, err := transA.AcceptPeer(getPeer(transB))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			_ = c.Close()
		}
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		c, err := transB.DialPeer(getPeer(transA))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			_ = c.Close()
		}
	}()

	wg.Wait()
}

func TestWrongConnect(t *testing.T) {
	transA, closeA := newTestServer(t, "A")
	defer closeA()
	transB, closeB := newTestServer(t, "B")
	defer closeB()
	transC, closeC := newTestServer(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(2)

	// a expects connection from B, but C is connecting
	go func() {
		defer wg.Done()
		_, err := transA.AcceptPeer(getPeer(transB))
		assert.Error(t, err)
	}()
	go func() {
		defer wg.Done()
		_, err := transC.DialPeer(getPeer(transA))
		assert.Error(t, err)
	}()

	wg.Wait()
}

func newTestDB(t require.TestingT) *peer.DB {
	db, err := peer.NewDB(mapdb.NewMapDB())
	require.NoError(t, err)
	return db
}

func newTestServer(t require.TestingT, name string) (*TCP, func()) {
	l := log.Named(name)

	services := service.New()
	services.Update(service.PeeringKey, "peering", name)
	local, err := peer.NewLocal(services, newTestDB(t))
	require.NoError(t, err)

	laddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	lis, err := net.ListenTCP("tcp", laddr)
	require.NoError(t, err)

	// enable TCP gossipping
	require.NoError(t, local.UpdateService(service.GossipKey, lis.Addr().Network(), lis.Addr().String()))

	srv := ServeTCP(local, lis, l)

	teardown := func() {
		srv.Close()
		_ = lis.Close()
	}
	return srv, teardown
}
