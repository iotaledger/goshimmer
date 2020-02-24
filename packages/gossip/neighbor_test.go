package gossip

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testData = []byte("foobar")

func TestNeighborClose(t *testing.T) {
	a, _, teardown := newPipe()
	defer teardown()

	n := newTestNeighbor("A", a)
	n.Listen()
	require.NoError(t, n.Close())
}

func TestNeighborCloseTwice(t *testing.T) {
	a, _, teardown := newPipe()
	defer teardown()

	n := newTestNeighbor("A", a)
	n.Listen()
	require.NoError(t, n.Close())
	require.NoError(t, n.Close())
}

func TestNeighborWrite(t *testing.T) {
	a, b, teardown := newPipe()
	defer teardown()

	neighborA := newTestNeighbor("A", a)
	defer neighborA.Close()
	neighborA.Listen()

	neighborB := newTestNeighbor("B", b)
	defer neighborB.Close()

	var count uint32
	neighborB.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
		assert.Equal(t, testData, data)
		atomic.AddUint32(&count, 1)
	}))
	neighborB.Listen()

	_, err := neighborA.Write(testData)
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return atomic.LoadUint32(&count) == 1 }, time.Second, 10*time.Millisecond)
}

func TestNeighborParallelWrite(t *testing.T) {
	a, b, teardown := newPipe()
	defer teardown()

	neighborA := newTestNeighbor("A", a)
	defer neighborA.Close()
	neighborA.Listen()

	neighborB := newTestNeighbor("B", b)
	defer neighborB.Close()

	var count uint32
	neighborB.Events.ReceiveMessage.Attach(events.NewClosure(func(data []byte) {
		assert.Equal(t, testData, data)
		atomic.AddUint32(&count, 1)
	}))
	neighborB.Listen()

	var (
		wg       sync.WaitGroup
		expected uint32
	)
	wg.Add(2)

	// Writer 1
	go func() {
		defer wg.Done()
		for i := 0; i < neighborQueueSize; i++ {
			l, err := neighborA.Write(testData)
			if err == ErrNeighborQueueFull || l == 0 {
				continue
			}
			assert.NoError(t, err)
			atomic.AddUint32(&expected, 1)
		}
	}()
	// Writer 2
	go func() {
		defer wg.Done()
		for i := 0; i < neighborQueueSize; i++ {
			l, err := neighborA.Write(testData)
			if err == ErrNeighborQueueFull || l == 0 {
				continue
			}
			assert.NoError(t, err)
			atomic.AddUint32(&expected, 1)
		}
	}()

	wg.Wait()

	done := func() bool {
		actual := atomic.LoadUint32(&count)
		return expected == actual
	}
	assert.Eventually(t, done, time.Second, 10*time.Millisecond)
}

func newTestNeighbor(name string, conn net.Conn) *Neighbor {
	return NewNeighbor(newTestPeer(name, conn.LocalAddr()), conn, log.Named(name))
}

func newTestPeer(name string, addr net.Addr) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, addr.Network(), addr.String())
	services.Update(service.GossipKey, addr.Network(), addr.String())

	return peer.NewPeer([]byte(name), services)
}

func newPipe() (net.Conn, net.Conn, func()) {
	a, b := net.Pipe()
	teardown := func() {
		_ = a.Close()
		_ = b.Close()
	}
	return a, b, teardown
}
