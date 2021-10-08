package gossip

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const graceTime = 10 * time.Millisecond

var (
	log             = logger.NewExampleLogger("gossip")
	testMessageData = []byte("testMsg")
)

func loadTestMessage(tangle.MessageID) ([]byte, error) { return testMessageData, nil }

func TestClose(t *testing.T) {
	_, teardown, _ := newMockedManager(t, "A")
	teardown()
}

func TestClosedConnection(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newMockedManager(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	// connect in the following way
	// B -> A
	mgrA.On("neighborAdded", mock.Anything).Once()
	mgrB.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()

	// A drops B
	err := mgrA.DropNeighbor(peerB.ID(), NeighborsGroupAuto)
	require.NoError(t, err)
	time.Sleep(graceTime)

	// the events should be there even before we close
	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestP2PSend(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	mgrB, closeB, peerB := newMockedManager(t, "B")

	var wg sync.WaitGroup
	wg.Add(2)

	// connect in the following way
	// B -> A
	mgrA.On("neighborAdded", mock.Anything).Once()
	mgrB.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrB.On("messageReceived", &MessageReceivedEvent{
		Data: testMessageData,
		Peer: peerA,
	}).Once()

	mgrA.SendMessage(testMessageData)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	// the events should be there even before we close
	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestP2PSendTwice(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	mgrB, closeB, peerB := newMockedManager(t, "B")

	var wg sync.WaitGroup
	wg.Add(2)

	// connect in the following way
	// B -> A
	mgrA.On("neighborAdded", mock.Anything).Once()
	mgrB.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrB.On("messageReceived", &MessageReceivedEvent{
		Data: testMessageData,
		Peer: peerA,
	}).Twice()

	mgrA.SendMessage(testMessageData)
	time.Sleep(1 * time.Second) // wait a bit between the sends, to test timeouts
	mgrA.SendMessage(testMessageData)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	// the events should be there even before we close
	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestBroadcast(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	mgrB, closeB, peerB := newMockedManager(t, "B")
	mgrC, closeC, peerC := newMockedManager(t, "C")

	var wg sync.WaitGroup
	wg.Add(4)

	// connect in the following way
	// B -> A <- C
	mgrA.On("neighborAdded", mock.Anything).Twice()
	mgrB.On("neighborAdded", mock.Anything).Once()
	mgrC.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerC, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	event := &MessageReceivedEvent{Data: testMessageData, Peer: peerA}
	mgrB.On("messageReceived", event).Once()
	mgrC.On("messageReceived", event).Once()

	mgrA.SendMessage(testMessageData)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()
	mgrC.On("neighborRemoved", mock.Anything).Once()

	closeA()
	closeB()
	closeC()
	time.Sleep(graceTime)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
	mgrC.AssertExpectations(t)
}

func TestSingleSend(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	mgrB, closeB, peerB := newMockedManager(t, "B")
	mgrC, closeC, peerC := newMockedManager(t, "C")

	var wg sync.WaitGroup
	wg.Add(4)

	// connect in the following way
	// B -> A <- C
	mgrA.On("neighborAdded", mock.Anything).Twice()
	mgrB.On("neighborAdded", mock.Anything).Once()
	mgrC.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerC, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	// only mgr should receive the message
	mgrB.On("messageReceived", &MessageReceivedEvent{Data: testMessageData, Peer: peerA}).Once()

	// A sends the message only to B
	mgrA.SendMessage(testMessageData, peerB.ID())
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()
	mgrC.On("neighborRemoved", mock.Anything).Once()

	closeA()
	closeB()
	closeC()
	time.Sleep(graceTime)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
	mgrC.AssertExpectations(t)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	mgrA, closeA, _ := newMockedManager(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newMockedManager(t, "B")
	defer closeB()

	mgrA.On("connectionFailed", peerB, mock.Anything).Once()

	err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
	assert.Error(t, err)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestMessageRequest(t *testing.T) {
	mgrA, closeA, peerA := newMockedManager(t, "A")
	mgrB, closeB, peerB := newMockedManager(t, "B")

	var wg sync.WaitGroup
	wg.Add(2)

	// connect in the following way
	// B -> A
	mgrA.On("neighborAdded", mock.Anything).Once()
	mgrB.On("neighborAdded", mock.Anything).Once()

	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	id := tangle.MessageID{}

	// mgrA should eventually receive the message
	mgrA.On("messageReceived", &MessageReceivedEvent{Data: testMessageData, Peer: peerB}).Once()

	b, err := proto.Marshal(&pb.MessageRequest{Id: id[:]})
	require.NoError(t, err)
	mgrA.RequestMessage(b)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestDropNeighbor(t *testing.T) {
	mgrA, closeA, peerA := newTestManager(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTestManager(t, "B")
	defer closeB()

	// establish connection
	connect := func() {
		var wg sync.WaitGroup
		signal := events.NewClosure(func(_ *Neighbor) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Attach(signal)
		defer mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Detach(signal)
		mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Attach(signal)
		defer mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Detach(signal)

		go func() { assert.NoError(t, mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)) }()
		go func() { assert.NoError(t, mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupAuto)) }()
		wg.Wait() // wait until the events were triggered and the peers are connected
	}
	// close connection
	disconnect := func() {
		var wg sync.WaitGroup
		signal := events.NewClosure(func(_ *Neighbor) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborRemoved.Attach(signal)
		defer mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborRemoved.Detach(signal)
		mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborRemoved.Attach(signal)
		defer mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborRemoved.Detach(signal)

		// assure that no DropNeighbor calls are leaking
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = mgrA.DropNeighbor(peerB.ID(), NeighborsGroupAuto)
		}()
		go func() {
			defer wg.Done()
			_ = mgrB.DropNeighbor(peerA.ID(), NeighborsGroupAuto)
		}()
		wg.Wait() // wait until the events were triggered and the go routines are done
	}

	// drop and connect many many times
	for i := 0; i < 100; i++ {
		connect()
		assert.NotEmpty(t, mgrA.AllNeighbors())
		assert.NotEmpty(t, mgrB.AllNeighbors())
		disconnect()
		assert.Empty(t, mgrA.AllNeighbors())
		assert.Empty(t, mgrB.AllNeighbors())
	}
}

func TestDropNeighborDifferentGroup(t *testing.T) {
	mgrA, closeA, peerA := newTestManager(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTestManager(t, "B")
	defer closeB()

	// establish connection
	connect := func() {
		var wg sync.WaitGroup
		signal := events.NewClosure(func(_ *Neighbor) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.NeighborsEvents(NeighborsGroupManual).NeighborAdded.Attach(signal)
		defer mgrA.NeighborsEvents(NeighborsGroupManual).NeighborAdded.Detach(signal)
		mgrB.NeighborsEvents(NeighborsGroupManual).NeighborAdded.Attach(signal)
		defer mgrB.NeighborsEvents(NeighborsGroupManual).NeighborAdded.Detach(signal)

		go func() { assert.NoError(t, mgrA.AddInbound(context.Background(), peerB, NeighborsGroupManual)) }()
		go func() { assert.NoError(t, mgrB.AddOutbound(context.Background(), peerA, NeighborsGroupManual)) }()
		wg.Wait() // wait until the events were triggered and the peers are connected
	}
	// close connection
	disconnect := func() {
		var wg sync.WaitGroup
		// assure that no DropNeighbor calls are leaking
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = mgrA.DropNeighbor(peerB.ID(), NeighborsGroupAuto)
		}()
		go func() {
			defer wg.Done()
			_ = mgrB.DropNeighbor(peerA.ID(), NeighborsGroupAuto)
		}()
		wg.Wait() // wait until the events were triggered and the go routines are done
	}
	connect()
	assert.NotEmpty(t, mgrA.AllNeighbors())
	assert.NotEmpty(t, mgrB.AllNeighbors())
	// drop many many times
	for i := 0; i < 100; i++ {
		disconnect()
		assert.NotEmpty(t, mgrA.AllNeighbors())
		assert.NotEmpty(t, mgrB.AllNeighbors())
	}
}

func newTestDB(t require.TestingT) *peer.DB {
	db, err := peer.NewDB(mapdb.NewMapDB())
	require.NoError(t, err)
	return db
}

func newTestManager(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	l := log.Named(name)

	host, err := libp2p.New(context.Background(), libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	lis := host.Network().ListenAddresses()[0]

	services := service.New()
	services.Update(service.PeeringKey, "peering", 0)
	tcpPortStr, err := lis.ValueForProtocol(multiaddr.P_TCP)
	require.NoError(t, err)
	tcpPort, err := strconv.Atoi(tcpPortStr)
	require.NoError(t, err)
	services.Update(service.GossipKey, "tcp", tcpPort)

	ipAddr, err := lis.ValueForProtocol(multiaddr.P_IP4)
	local, err := peer.NewLocal(net.ParseIP(ipAddr), services, newTestDB(t))
	require.NoError(t, err)

	// start the actual gossipping
	mgr := NewManager(host, local, loadTestMessage, l)

	return mgr, mgr.Stop, local.Peer
}

func newMockedManager(t *testing.T, name string) (*mockedManager, func(), *peer.Peer) {
	mgr, detach, p := newTestManager(t, name)
	return mockManager(t, mgr), detach, p
}

func mockManager(t mock.TestingT, mgr *Manager) *mockedManager {
	e := &mockedManager{Manager: mgr}
	e.Test(t)

	e.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Attach(events.NewClosure(e.neighborAdded))
	e.NeighborsEvents(NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(e.neighborRemoved))
	e.Events().MessageReceived.Attach(events.NewClosure(e.messageReceived))

	return e
}

type mockedManager struct {
	mock.Mock
	*Manager
}

func (e *mockedManager) neighborAdded(n *Neighbor)                { e.Called(n) }
func (e *mockedManager) neighborRemoved(n *Neighbor)              { e.Called(n) }
func (e *mockedManager) messageReceived(ev *MessageReceivedEvent) { e.Called(ev) }
