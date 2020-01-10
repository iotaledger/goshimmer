package gossip

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/server"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const graceTime = 10 * time.Millisecond

var (
	log        = logger.NewExampleLogger("gossip")
	testTxData = []byte("testTx")
)

func getTestTransaction([]byte) ([]byte, error) { return testTxData, nil }

func getTCPAddress(t require.TestingT) string {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	require.NoError(t, err)
	lis, err := net.ListenTCP("tcp", tcpAddr)
	require.NoError(t, err)

	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	return addr
}

func newTestManager(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	l := log.Named(name)
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)

	// enable TCP gossipping
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", getTCPAddress(t)))

	mgr := NewManager(local, getTestTransaction, l)

	srv, err := server.ListenTCP(local, l)
	require.NoError(t, err)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, srv.LocalAddr().Network(), srv.LocalAddr().String()))

	// start the actual gossipping
	mgr.Start(srv)

	detach := func() {
		mgr.Close()
		srv.Close()
		db.Close()
	}
	return mgr, detach, &local.Peer
}

func TestClose(t *testing.T) {
	_, detach := newEventMock(t)
	defer detach()

	_, teardown, _ := newTestManager(t, "A")
	teardown()
}

func TestClosedConnection(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTestManager(t, "B")
	defer closeB()

	connections := 2
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A
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

	e.On("neighborRemoved", peerA).Once()
	e.On("neighborRemoved", peerB).Once()

	// A drops B
	err := mgrA.DropNeighbor(peerB.ID())
	require.NoError(t, err)
	time.Sleep(graceTime)

	// the events should be there even before we close
	e.AssertExpectations(t)
}

func TestP2PSend(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	mgrB, closeB, peerB := newTestManager(t, "B")

	connections := 2
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A
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

	e.On("transactionReceived", &TransactionReceivedEvent{
		Data: testTxData,
		Peer: peerA,
	}).Once()

	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)

	e.On("neighborRemoved", peerA).Once()
	e.On("neighborRemoved", peerB).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	e.AssertExpectations(t)
}

func TestP2PSendTwice(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	mgrB, closeB, peerB := newTestManager(t, "B")

	connections := 2
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A
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

	e.On("transactionReceived", &TransactionReceivedEvent{
		Data: testTxData,
		Peer: peerA,
	}).Twice()

	mgrA.SendTransaction(testTxData)
	time.Sleep(1 * time.Second) // wait a bit between the sends, to test timeouts
	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)

	e.On("neighborRemoved", peerA).Once()
	e.On("neighborRemoved", peerB).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	e.AssertExpectations(t)
}

func TestBroadcast(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	mgrB, closeB, peerB := newTestManager(t, "B")
	mgrC, closeC, peerC := newTestManager(t, "C")

	connections := 4
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A <- C
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

	e.On("transactionReceived", &TransactionReceivedEvent{
		Data: testTxData,
		Peer: peerA,
	}).Twice()

	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)

	e.On("neighborRemoved", peerA).Twice()
	e.On("neighborRemoved", peerB).Once()
	e.On("neighborRemoved", peerC).Once()

	closeA()
	closeB()
	closeC()
	time.Sleep(graceTime)

	e.AssertExpectations(t)
}

func TestSingleSend(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	mgrB, closeB, peerB := newTestManager(t, "B")
	mgrC, closeC, peerC := newTestManager(t, "C")

	connections := 4
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A <- C
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

	e.On("transactionReceived", &TransactionReceivedEvent{
		Data: testTxData,
		Peer: peerA,
	}).Once()

	// A sends the transaction only to B
	mgrA.SendTransaction(testTxData, peerB.ID())
	time.Sleep(graceTime)

	e.On("neighborRemoved", peerA).Twice()
	e.On("neighborRemoved", peerB).Once()
	e.On("neighborRemoved", peerC).Once()

	closeA()
	closeB()
	closeC()
	time.Sleep(graceTime)

	e.AssertExpectations(t)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, _ := newTestManager(t, "A")
	defer closeA()
	_, closeB, peerB := newTestManager(t, "B")
	defer closeB()

	e.On("connectionFailed", peerB).Once()

	err := mgrA.AddInbound(peerB)
	assert.Error(t, err)

	e.AssertExpectations(t)
}

func TestTxRequest(t *testing.T) {
	e, detach := newEventMock(t)
	defer detach()

	mgrA, closeA, peerA := newTestManager(t, "A")
	mgrB, closeB, peerB := newTestManager(t, "B")

	connections := 2
	e.On("neighborAdded", mock.Anything).Times(connections)

	var wg sync.WaitGroup
	wg.Add(connections)

	// connect in the following way
	// B -> A
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

	txHash := []byte("Hello!")

	e.On("transactionReceived", &TransactionReceivedEvent{
		Data: testTxData,
		Peer: peerB,
	}).Once()

	b, err := proto.Marshal(&pb.TransactionRequest{Hash: txHash})
	require.NoError(t, err)
	mgrA.RequestTransaction(b)
	time.Sleep(graceTime)

	e.On("neighborRemoved", peerA).Once()
	e.On("neighborRemoved", peerB).Once()

	closeA()
	closeB()
	time.Sleep(graceTime)

	e.AssertExpectations(t)
}

func newEventMock(t mock.TestingT) (*eventMock, func()) {
	e := &eventMock{}
	e.Test(t)

	connectionFailedC := events.NewClosure(e.connectionFailed)
	neighborAddedC := events.NewClosure(e.neighborAdded)
	neighborRemoved := events.NewClosure(e.neighborRemoved)
	transactionReceivedC := events.NewClosure(e.transactionReceived)

	Events.ConnectionFailed.Attach(connectionFailedC)
	Events.NeighborAdded.Attach(neighborAddedC)
	Events.NeighborRemoved.Attach(neighborRemoved)
	Events.TransactionReceived.Attach(transactionReceivedC)

	return e, func() {
		Events.ConnectionFailed.Detach(connectionFailedC)
		Events.NeighborAdded.Detach(neighborAddedC)
		Events.NeighborRemoved.Detach(neighborRemoved)
		Events.TransactionReceived.Detach(transactionReceivedC)
	}
}

type eventMock struct {
	mock.Mock
}

func (e *eventMock) connectionFailed(p *peer.Peer)                    { e.Called(p) }
func (e *eventMock) neighborAdded(n *Neighbor)                        { e.Called(n) }
func (e *eventMock) neighborRemoved(p *peer.Peer)                     { e.Called(p) }
func (e *eventMock) transactionReceived(ev *TransactionReceivedEvent) { e.Called(ev) }
