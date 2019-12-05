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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const graceTime = 5 * time.Millisecond

var logger *zap.SugaredLogger

func init() {
	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	logger = l.Sugar()
}
func testGetTransaction([]byte) ([]byte, error) {
	tx := &pb.TransactionRequest{
		Hash: []byte("testTx"),
	}
	b, _ := proto.Marshal(tx)
	return b, nil
}

func newTest(t require.TestingT, name string) (*Manager, func(), *peer.Peer) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal("peering", name, db)
	require.NoError(t, err)
	require.NoError(t, local.UpdateService(service.GossipKey, "tcp", "localhost:0"))

	trans, err := transport.Listen(local, log)
	require.NoError(t, err)

	mgr := NewManager(trans, log, testGetTransaction)

	// update the service with the actual address
	require.NoError(t, local.UpdateService(service.GossipKey, trans.LocalAddr().Network(), trans.LocalAddr().String()))

	teardown := func() {
		trans.Close()
		db.Close()
	}
	return mgr, teardown, &local.Peer
}

func TestClose(t *testing.T) {
	_, teardown, _ := newTest(t, "A")
	teardown()
}

func TestUnicast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	tx := &pb.Transaction{Body: []byte("Hello!")}

	triggered := make(chan struct{}, 1)
	mgrB.Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		require.Empty(t, triggered) // only once
		assert.Equal(t, tx.GetBody(), ev.Body)
		assert.Equal(t, peerA, ev.Peer)
		triggered <- struct{}{}
	}))

	b, err := proto.Marshal(tx)
	require.NoError(t, err)
	mgrA.Send(b)

	// eventually the event should be triggered
	assert.Eventually(t, func() bool { return len(triggered) >= 1 }, time.Second, 10*time.Millisecond)
}

func TestBroadcast(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()
	mgrC, closeC, peerC := newTest(t, "C")
	defer closeC()

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerC, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.addNeighbor(peerA, mgrC.trans.DialPeer)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	tx := &pb.Transaction{Body: []byte("Hello!")}

	triggeredB := make(chan struct{}, 1)
	mgrB.Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		require.Empty(t, triggeredB) // only once
		assert.Equal(t, tx.GetBody(), ev.Body)
		assert.Equal(t, peerA, ev.Peer)
		triggeredB <- struct{}{}
	}))

	triggeredC := make(chan struct{}, 1)
	mgrC.Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		require.Empty(t, triggeredC) // only once
		assert.Equal(t, tx.GetBody(), ev.Body)
		assert.Equal(t, peerA, ev.Peer)
		triggeredC <- struct{}{}
	}))

	b, err := proto.Marshal(tx)
	assert.NoError(t, err)
	mgrA.Send(b)

	// eventually the events should be triggered
	success := func() bool {
		return len(triggeredB) >= 1 && len(triggeredC) >= 1
	}
	assert.Eventually(t, success, time.Second, 10*time.Millisecond)
}

func TestDropUnsuccessfulAccept(t *testing.T) {
	mgrA, closeA, _ := newTest(t, "A")
	defer closeA()
	_, closeB, peerB := newTest(t, "B")
	defer closeB()

	triggered := make(chan struct{}, 1)
	mgrA.Events.DropNeighbor.Attach(events.NewClosure(func(ev *DropNeighborEvent) {
		require.Empty(t, triggered) // only once
		assert.Equal(t, peerB, ev.Peer)
		triggered <- struct{}{}
	}))

	err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
	assert.Error(t, err)

	// eventually the event should be triggered
	assert.Eventually(t, func() bool { return len(triggered) >= 1 }, time.Second, 10*time.Millisecond)
}

func TestTxRequest(t *testing.T) {
	mgrA, closeA, peerA := newTest(t, "A")
	defer closeA()
	mgrB, closeB, peerB := newTest(t, "B")
	defer closeB()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := mgrA.addNeighbor(peerB, mgrA.trans.AcceptPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrA.neighborhood.Len())
	}()
	go func() {
		defer wg.Done()
		err := mgrB.addNeighbor(peerA, mgrB.trans.DialPeer)
		assert.NoError(t, err)
		logger.Debugw("Len", "len", mgrB.neighborhood.Len())
	}()

	wg.Wait()

	tx := &pb.TransactionRequest{
		Hash: []byte("Hello!"),
	}
	b, err := proto.Marshal(tx)
	assert.NoError(t, err)

	sendChan := make(chan struct{})
	sendSuccess := false

	mgrA.Events.NewTransaction.Attach(events.NewClosure(func(ev *NewTransactionEvent) {
		logger.Debugw("New TX Event triggered", "data", ev.Body, "from", ev.Peer.ID().String())
		assert.Equal(t, []byte("testTx"), ev.Body)
		assert.Equal(t, peerB, ev.Peer)
		sendChan <- struct{}{}
	}))

	mgrA.RequestTransaction(b)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-sendChan:
		sendSuccess = true
	case <-timer.C:
		sendSuccess = false
	}

	assert.True(t, sendSuccess)
}
