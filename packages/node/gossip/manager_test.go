package gossip

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/logger"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	gp "github.com/iotaledger/goshimmer/packages/node/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/node/libp2putil"
	"github.com/iotaledger/goshimmer/packages/node/p2p"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

const graceTime = 10 * time.Millisecond

var (
	log           = logger.NewExampleLogger("gossip")
	testBlockData = []byte("testBlk")
)

func loadTestBlock(tangleold.BlockID) ([]byte, error) { return testBlockData, nil }

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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrA.On("neighborRemoved", mock.Anything).Once()
	mgrB.On("neighborRemoved", mock.Anything).Once()

	// A drops B
	err := mgrA.p2pManager.DropNeighbor(peerB.ID(), p2p.NeighborsGroupAuto)
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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrB.On("blockReceived", &BlockReceivedEvent{
		Data: testBlockData,
		Peer: peerA,
	}).Once()

	mgrA.SendBlock(testBlockData)
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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	mgrB.On("blockReceived", &BlockReceivedEvent{
		Data: testBlockData,
		Peer: peerA,
	}).Twice()

	mgrA.SendBlock(testBlockData)
	time.Sleep(1 * time.Second) // wait a bit between the sends, to test timeouts
	mgrA.SendBlock(testBlockData)
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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.p2pManager.AddInbound(context.Background(), peerC, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	event := &BlockReceivedEvent{Data: testBlockData, Peer: peerA}
	mgrB.On("blockReceived", event).Once()
	mgrC.On("blockReceived", event).Once()

	mgrA.SendBlock(testBlockData)
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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrA.p2pManager.AddInbound(context.Background(), peerC, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := mgrC.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	// only mgr should receive the block
	mgrB.On("blockReceived", &BlockReceivedEvent{Data: testBlockData, Peer: peerA}).Once()

	// A sends the block only to B
	mgrA.SendBlock(testBlockData, peerB.ID())
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

	err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
	assert.Error(t, err)

	mgrA.AssertExpectations(t)
	mgrB.AssertExpectations(t)
}

func TestBlockRequest(t *testing.T) {
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
		err := mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()
	time.Sleep(graceTime)
	go func() {
		defer wg.Done()
		err := mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto)
		assert.NoError(t, err)
	}()

	// wait for the connections to establish
	wg.Wait()

	id := tangleold.BlockID{}

	// mgrA should eventually receive the block
	mgrA.On("blockReceived", &BlockReceivedEvent{Data: testBlockData, Peer: peerB}).Once()

	b, err := proto.Marshal(&gp.BlockRequest{Id: id.Bytes()})
	require.NoError(t, err)
	mgrA.RequestBlock(b)
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
		signalA := event.NewClosure(func(_ *p2p.NeighborAddedEvent) { wg.Done() })
		signalB := event.NewClosure(func(_ *p2p.NeighborAddedEvent) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Hook(signalA)
		defer mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Detach(signalA)
		mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Hook(signalB)
		defer mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Detach(signalB)

		go func() {
			assert.NoError(t, mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupAuto))
		}()
		time.Sleep(graceTime)
		go func() {
			assert.NoError(t, mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupAuto))
		}()
		wg.Wait() // wait until the events were triggered and the peers are connected
	}
	// close connection
	disconnect := func() {
		var wg sync.WaitGroup
		signal := event.NewClosure(func(_ *p2p.NeighborRemovedEvent) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Hook(signal)
		defer mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Detach(signal)
		mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Hook(signal)
		defer mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Detach(signal)

		// assure that no p2pManager.DropNeighbor calls are leaking
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = mgrA.p2pManager.DropNeighbor(peerB.ID(), p2p.NeighborsGroupAuto)
		}()
		go func() {
			defer wg.Done()
			_ = mgrB.p2pManager.DropNeighbor(peerA.ID(), p2p.NeighborsGroupAuto)
		}()
		wg.Wait() // wait until the events were triggered and the go routines are done
	}

	// drop and connect many many times
	for i := 0; i < 100; i++ {
		connect()
		assert.NotEmpty(t, mgrA.p2pManager.AllNeighbors())
		assert.NotEmpty(t, mgrB.p2pManager.AllNeighbors())
		disconnect()
		assert.Empty(t, mgrA.p2pManager.AllNeighbors())
		assert.Empty(t, mgrB.p2pManager.AllNeighbors())
	}
}

func TestManager(t *testing.T) {
	testMgrs := newTestManagers(t, false /* doMock */, t.Name()+"_A", t.Name()+"_B")
	mgrA, closeA, peerA := testMgrs[0].manager, testMgrs[0].close, testMgrs[0].peer
	mgrB, closeB, peerB := testMgrs[1].manager, testMgrs[1].close, testMgrs[1].peer
	defer closeA()
	defer closeB()

	// establish connection
	connect := func() {
		var wg sync.WaitGroup
		signal := event.NewClosure(func(_ *p2p.NeighborAddedEvent) { wg.Done() })
		// we are expecting two signals
		wg.Add(2)

		// signal as soon as the neighbor is added
		mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupManual).NeighborAdded.Hook(signal)
		defer mgrA.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupManual).NeighborAdded.Detach(signal)
		mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupManual).NeighborAdded.Hook(signal)
		defer mgrB.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupManual).NeighborAdded.Detach(signal)

		go func() {
			assert.NoError(t, mgrA.p2pManager.AddInbound(context.Background(), peerB, p2p.NeighborsGroupManual))
		}()
		time.Sleep(graceTime)
		go func() {
			assert.NoError(t, mgrB.p2pManager.AddOutbound(context.Background(), peerA, p2p.NeighborsGroupManual))
		}()
		wg.Wait() // wait until the events were triggered and the peers are connected
	}
	// close connection
	disconnect := func() {
		var wg sync.WaitGroup
		// assure that no p2pManager.DropNeighbor calls are leaking
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = mgrA.p2pManager.DropNeighbor(peerB.ID(), p2p.NeighborsGroupAuto)
		}()
		go func() {
			defer wg.Done()
			_ = mgrB.p2pManager.DropNeighbor(peerA.ID(), p2p.NeighborsGroupAuto)
		}()
		wg.Wait() // wait until the events were triggered and the go routines are done
	}
	connect()
	assert.NotEmpty(t, mgrA.p2pManager.AllNeighbors())
	assert.NotEmpty(t, mgrB.p2pManager.AllNeighbors())
	// drop many many times
	for i := 0; i < 100; i++ {
		disconnect()
		assert.NotEmpty(t, mgrA.p2pManager.AllNeighbors())
		assert.NotEmpty(t, mgrB.p2pManager.AllNeighbors())
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
		err = local.UpdateService(service.P2PKey, "tcp", tcpPort)
		require.NoError(t, err)

		// start the actual gossipping
		p2pMgr := p2p.NewManager(hst, local, l)
		mgr := NewManager(p2pMgr, loadTestBlock, l)
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

	e.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Hook(event.NewClosure(e.neighborAdded))
	e.p2pManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Hook(event.NewClosure(e.neighborRemoved))
	e.Events.BlockReceived.Hook(event.NewClosure(e.blockReceived))

	return e
}

type mockedManager struct {
	mock.Mock
	*Manager
}

func (e *mockedManager) neighborAdded(event *p2p.NeighborAddedEvent)     { e.Called(event.Neighbor) }
func (e *mockedManager) neighborRemoved(event *p2p.NeighborRemovedEvent) { e.Called(event.Neighbor) }
func (e *mockedManager) blockReceived(event *BlockReceivedEvent)         { e.Called(event) }
