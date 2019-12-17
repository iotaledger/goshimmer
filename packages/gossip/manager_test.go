package gossip

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/peer/service"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/gossip/transport"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const graceTime = 10 * time.Millisecond

var (
	logger    *zap.SugaredLogger
	eventMock mock.Mock

	testTxData = []byte("testTx")
)

func transactionReceivedEvent(ev *TransactionReceivedEvent) { eventMock.Called(ev) }
func neighborDroppedEvent(ev *NeighborDroppedEvent)         { eventMock.Called(ev) }

// assertEvents initializes the mock and asserts the expectations
func assertEvents(t *testing.T) func() {
	eventMock = mock.Mock{}
	return func() {
		if !t.Failed() {
			eventMock.AssertExpectations(t)
		}
	}
}

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()

	// mock the events triggered by the gossip
	Events.TransactionReceived.Attach(events.NewClosure(transactionReceivedEvent))
	Events.NeighborDropped.Attach(events.NewClosure(neighborDroppedEvent))
}

func getTestTransaction([]byte) ([]byte, error) {
	return testTxData, nil
}

func newTest(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	l := logger.Named(name)
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", "localhost:0"))

	trans, err := transport.Listen(local, l)
	require.NoError(t, err)

	mgr := NewManager(trans, l, getTestTransaction)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		mgr.Close()
		trans.Close()
		db.Close()
	}
	return mgr, teardown, &local.Peer
}

func TestClose(t *testing.T) {
	defer assertEvents(t)()

	_, teardown, _ := newTest(t, "A")
	teardown()
}

func TestClosedConnection(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

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

	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()

	// A drops B
	err := mgrA.DropNeighbor(peerB.ID())
	require.NoError(t, err)
	time.Sleep(graceTime)

	// the events should be there even before we close
	eventMock.AssertExpectations(t)
}

func TestP2PSend(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

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

	eventMock.On("transactionReceivedEvent", &TransactionReceivedEvent{
		Body: testTxData,
		Peer: peerA,
	}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()

	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)
}

func TestP2PSendTwice(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

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

	eventMock.On("transactionReceivedEvent", &TransactionReceivedEvent{
		Body: testTxData,
		Peer: peerA,
	}).Twice()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()

	mgrA.SendTransaction(testTxData)
	time.Sleep(1 * time.Second) // wait a bit between the sends, to test timeouts
	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)
}

func TestBroadcast(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()
	mgrC, closeC, peerC := newTest(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(4)

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

	eventMock.On("transactionReceivedEvent", &TransactionReceivedEvent{
		Body: testTxData,
		Peer: peerA,
	}).Twice()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Twice()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerC}).Once()

	mgrA.SendTransaction(testTxData)
	time.Sleep(graceTime)
}

func TestSingleSend(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()
	mgrC, closeC, peerC := newTest(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(4)

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

	eventMock.On("transactionReceivedEvent", &TransactionReceivedEvent{
		Body: testTxData,
		Peer: peerA,
	}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Twice()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerC}).Once()

	// A sends the transaction only to B
	mgrA.SendTransaction(testTxData, peerB.ID())
	time.Sleep(graceTime)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, _ := newTest(t, "A")
	defer closeA()
	_, closeB, peerB := newTest(t, "B")
	defer closeB()

	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{
		Peer: peerB,
	}).Once()

	err := mgrA.AddInbound(peerB)
	assert.Error(t, err)
}

func TestTxRequest(t *testing.T) {
	defer assertEvents(t)()

	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

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

	eventMock.On("transactionReceivedEvent", &TransactionReceivedEvent{
		Body: testTxData,
		Peer: peerB,
	}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerA}).Once()
	eventMock.On("neighborDroppedEvent", &NeighborDroppedEvent{Peer: peerB}).Once()

	b, err := proto.Marshal(&pb.TransactionRequest{Hash: txHash})
	require.NoError(t, err)
	mgrA.RequestTransaction(b)
	time.Sleep(graceTime)
}
