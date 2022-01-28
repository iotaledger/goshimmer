package gossip

import (
	"context"
	"fmt"
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
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/libp2putil"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const graceTime = 10 * time.Millisecond

var (
	log             = logger.NewExampleLogger("gossip")
	testMessageData = []byte("testMsg")
)

func loadTestMessage(tangle.MessageID) ([]byte, error) { return testMessageData, nil }

func TestClose(t *testing.T) {
	testMgrs := newTestManagers(t, true /* doMock */, "A")
	testMgrs[0].close()
}

func TestClosedConnection(t *testing.T) {
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
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
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer

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
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
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
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B", t.Name()+"_C")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer
	mgrC, closeC, peerC := testMgrs[2].mockManager, testMgrs[2].close, testMgrs[2].peer
	defer closeA()
	defer closeB()

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
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B", t.Name()+"_C")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer
	mgrC, closeC, peerC := testMgrs[2].mockManager, testMgrs[2].close, testMgrs[2].peer

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
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, _ := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
	defer closeB()

	err := mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)
	assert.Error(t, err)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestMessageRequest(t *testing.T) {
	testMgrs := newTestManagers(t, true /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].mockManager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].mockManager, testMgrs[1].close, testMgrs[1].peer

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
	testMgrs := newTestManagers(t, false /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].manager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].manager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
	defer closeB()

	// establish connection
	connect := func() {
		var wg sync.WaitGroup
		signalA := events.NewClosure(func(_ *Neighbor) { wg.Done() })
		signalB := events.NewClosure(func(_ *Neighbor) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Attach(signalA)
		defer mgrA.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Detach(signalA)
		mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Attach(signalB)
		defer mgrB.NeighborsEvents(NeighborsGroupAuto).NeighborAdded.Detach(signalB)

		go func() { assert.NoError(t, mgrA.AddInbound(context.Background(), peerB, NeighborsGroupAuto)) }()
		time.Sleep(graceTime)
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
	testMgrs := newTestManagers(t, false /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].manager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].manager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
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
		time.Sleep(graceTime)
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

type testManager struct {
	manager     *Manager
	mockManager *mockedManager
	close       func()
	peer        *peer.Peer
}

func newTestManagers(t testing.TB, doMock bool, names ...string) []*testManager {
	ctx := context.Background()
	mn := mocknet.New(ctx)
	var results []*testManager
	for _, name := range names {
		name := name
		l := log.Named(name)
		services := service.New()
		services.Update(service.PeeringKey, "peering", 0)
		local, err := peer.NewLocal(net.ParseIP("127.0.0.1"), services, newTestDB(t))
		require.NoError(t, err)
		ourPrivKey, err := local.Database().LocalPrivateKey()
		require.NoError(t, err)
		libp2pPrivKey, err := libp2putil.ToLibp2pPrivateKey(ourPrivKey)
		require.NoError(t, err)
		id, err := libp2ppeer.IDFromPrivateKey(libp2pPrivKey)
		require.NoError(t, err)
		suffix := id
		if len(id) > 8 {
			suffix = id[len(id)-8:]
		}
		blackholeIP6 := net.ParseIP("100::")
		ip := append(net.IP{}, blackholeIP6...)
		copy(ip[net.IPv6len-len(suffix):], suffix)
		addr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip6/%s/tcp/4242", ip))
		require.NoError(t, err)
		hst, err := mn.AddPeer(libp2pPrivKey, addr)
		require.NoError(t, err)
		lis := hst.Addrs()[0]
		tcpPortStr, err := lis.ValueForProtocol(multiaddr.P_TCP)
		require.NoError(t, err)
		tcpPort, err := strconv.Atoi(tcpPortStr)
		require.NoError(t, err)
		err = local.UpdateService(service.GossipKey, "tcp", tcpPort)
		require.NoError(t, err)

		// start the actual gossipping
		mgr := NewManager(hst, local, loadTestMessage, l)
		require.NoError(t, err)
		tearDown := func() {
			mgr.Stop()
			err := hst.Close()
			require.NoError(t, err)
		}
		var mockMgr *mockedManager
		if doMock {
			mockMgr = mockManager(t, mgr)
		}
		results = append(results, &testManager{
			manager:     mgr,
			mockManager: mockMgr,
			peer:        local.Peer,
			close:       tearDown,
		})

	}
	err := mn.LinkAll()
	require.NoError(t, err)
	err = mn.ConnectAllButSelf()
	require.NoError(t, err)
	return results
}

func mockManager(t testing.TB, mgr *Manager) *mockedManager {
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
