package gossip

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/database/mapdb"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const graceTime = 10 * time.Millisecond

var (
	log             = logger.NewExampleLogger("gossip")
	testMessageData = []byte("testMsg")
)

func loadTestMessage(message.Id) ([]byte, error) { return testMessageData, nil }

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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrA.On("neighborRemoved", peerB).Once()
	mgrB.On("neighborRemoved", peerA).Once()

	// A drops B
	err := mgrA.DropNeighbor(peerB.ID())
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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
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

	mgrA.On("neighborRemoved", peerB).Once()
	mgrB.On("neighborRemoved", peerA).Once()

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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
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

	mgrA.On("neighborRemoved", peerB).Once()
	mgrB.On("neighborRemoved", peerA).Once()

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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(peerC)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.AddOutbound(peerA)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	event := &MessageReceivedEvent{Data: testMessageData, Peer: peerA}
	mgrB.On("messageReceived", event).Once()
	mgrC.On("messageReceived", event).Once()

	mgrA.SendMessage(testMessageData)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", peerB).Once()
	mgrA.On("neighborRemoved", peerC).Once()
	mgrB.On("neighborRemoved", peerA).Once()
	mgrC.On("neighborRemoved", peerA).Once()

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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.AddInbound(peerC)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.AddOutbound(peerA)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	// only mgr should receive the message
	mgrB.On("messageReceived", &MessageReceivedEvent{Data: testMessageData, Peer: peerA}).Once()

	// A sends the message only to B
	mgrA.SendMessage(testMessageData, peerB.ID())
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", peerB).Once()
	mgrA.On("neighborRemoved", peerC).Once()
	mgrB.On("neighborRemoved", peerA).Once()
	mgrC.On("neighborRemoved", peerA).Once()

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

	err := mgrA.AddInbound(peerB)
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
		err := mgrA.AddInbound(peerB)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.AddOutbound(peerA)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	messageId := message.Id{}

	// mgrA should eventually receive the message
	mgrA.On("messageReceived", &MessageReceivedEvent{Data: testMessageData, Peer: peerB}).Once()

	b, err := proto.Marshal(&pb.MessageRequest{Id: messageId[:]})
	require.NoError(t, err)
	mgrA.RequestMessage(b)
	time.Sleep(graceTime)

	mgrA.On("neighborRemoved", peerB).Once()
	mgrB.On("neighborRemoved", peerA).Once()

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
		mgrA.Events().NeighborAdded.Attach(signal)
		defer mgrA.Events().NeighborAdded.Detach(signal)
		mgrB.Events().NeighborAdded.Attach(signal)
		defer mgrB.Events().NeighborAdded.Detach(signal)

		go func() { assert.NoError(t, mgrA.AddInbound(peerB)) }()
		go func() { assert.NoError(t, mgrB.AddOutbound(peerA)) }()
		wg.Wait() // wait until the events were triggered and the peers are connected
	}
	// close connection
	disconnect := func() {
		var wg sync.WaitGroup
		signal := events.NewClosure(func(_ *peer.Peer) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.Events().NeighborRemoved.Attach(signal)
		defer mgrA.Events().NeighborRemoved.Detach(signal)
		mgrB.Events().NeighborRemoved.Attach(signal)
		defer mgrB.Events().NeighborRemoved.Detach(signal)

		// assure that no DropNeighbor calls are leaking
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = mgrA.DropNeighbor(peerB.ID())
		}()
		go func() {
			defer wg.Done()
			_ = mgrB.DropNeighbor(peerA.ID())
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

func newTestDB(t require.TestingT) *peer.DB {
	db, err := peer.NewDB(mapdb.NewMapDB())
	require.NoError(t, err)
	return db
}

func newTestManager(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	l := log.Named(name)

	laddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	lis, err := net.ListenTCP("tcp", laddr)
	require.NoError(t, err)

	services := service.New()
	services.Update(service.PeeringKey, "peering", 0)
	services.Update(service.GossipKey, lis.Addr().Network(), lis.Addr().(*net.TCPAddr).Port)

	local, err := peer.NewLocal(lis.Addr().(*net.TCPAddr).IP, services, newTestDB(t))
	require.NoError(t, err)

	srv := server.ServeTCP(local, lis, l)

	// start the actual gossipping
	mgr := NewManager(local, loadTestMessage, l)
	mgr.Start(srv)

	detach := func() {
		mgr.Close()
		srv.Close()
		_ = lis.Close()
	}
	return mgr, detach, local.Peer
}

func newMockedManager(t *testing.T, name string) (*mockedManager, func(), *peer.Peer) {
	mgr, detach, p := newTestManager(t, name)
	return mockManager(t, mgr), detach, p
}

func mockManager(t mock.TestingT, mgr *Manager) *mockedManager {
	e := &mockedManager{Manager: mgr}
	e.Test(t)

	e.Events().ConnectionFailed.Attach(events.NewClosure(e.connectionFailed))
	e.Events().NeighborAdded.Attach(events.NewClosure(e.neighborAdded))
	e.Events().NeighborRemoved.Attach(events.NewClosure(e.neighborRemoved))
	e.Events().MessageReceived.Attach(events.NewClosure(e.messageReceived))

	return e
}

type mockedManager struct {
	mock.Mock
	*Manager
}

func (e *mockedManager) connectionFailed(p *peer.Peer, err error) { e.Called(p, err) }
func (e *mockedManager) neighborAdded(n *Neighbor)                { e.Called(n) }
func (e *mockedManager) neighborRemoved(p *peer.Peer)             { e.Called(p) }
func (e *mockedManager) messageReceived(ev *MessageReceivedEvent) { e.Called(ev) }
