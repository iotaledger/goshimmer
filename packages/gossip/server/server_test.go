package server

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graceTime = 5 * time.Millisecond

var log = logger.NewExampleLogger("server")

func getTCPAddress(t require.TestingT) string {
	laddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	require.NoError(t, err)
	lis, err := net.ListenTCP("tcp", laddr)
	require.NoError(t, err)

	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	return addr
}

func newTest(t require.TestingT, name string) (*TCP, func()) {
	l := log.Named(name)
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)

	// enable TCP gossipping
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", getTCPAddress(t)))

	trans, err := ListenTCP(local, l)
	require.NoError(t, err)

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return trans, teardown
}

func getPeer(t *TCP) *peer.Peer {
	return &t.local.Peer
}

func TestClose(t *testing.T) {
	_, teardown := newTest(t, "A")
	teardown()
}

func TestUnansweredAccept(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()

	_, err := transA.AcceptPeer(getPeer(transA))
	assert.Error(t, err)
}

func TestCloseWhileAccepting(t *testing.T) {
	transA, closeA := newTest(t, "A")

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
	transA, closeA := newTest(t, "A")
	defer closeA()

	// create peer with invalid gossip address
	services := getPeer(transA).Services().CreateRecord()
	services.Update(service.GossipKey, "tcp", "localhost:0")
	unreachablePeer := peer.NewPeer(getPeer(transA).PublicKey(), services)

	_, err := transA.DialPeer(unreachablePeer)
	assert.Error(t, err)
}

func TestNoHandshakeResponse(t *testing.T) {
	transA, closeA := newTest(t, "A")
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
	transA, closeA := newTest(t, "A")
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
	transA, closeA := newTest(t, "A")
	defer closeA()
	transB, closeB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c, err := transA.AcceptPeer(getPeer(transB))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			c.Close()
		}
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		c, err := transB.DialPeer(getPeer(transA))
		assert.NoError(t, err)
		if assert.NotNil(t, c) {
			c.Close()
		}
	}()

	wg.Wait()
}

func TestWrongConnect(t *testing.T) {
	transA, closeA := newTest(t, "A")
	defer closeA()
	transB, closeB := newTest(t, "B")
	defer closeB()
	transC, closeC := newTest(t, "C")
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
