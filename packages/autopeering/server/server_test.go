package server

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const graceTime = 5 * time.Millisecond

var log = logger.NewExampleLogger("server")

const (
	MPing MType = iota
	MPong
)

type Message interface {
	Type() MType
	Marshal() []byte
}

type Ping struct{}
type Pong struct{}

func (m *Ping) Type() MType     { return MPing }
func (m *Ping) Marshal() []byte { return append([]byte{}, byte(MPing)) }

func (m *Pong) Type() MType     { return MPong }
func (m *Pong) Marshal() []byte { return append([]byte{}, byte(MPong)) }

func sendPong(args mock.Arguments) {
	srv := args.Get(0).(*Server)
	addr := args.Get(1).(string)
	srv.Send(addr, new(Pong).Marshal())
}

var (
	pingMock *mock.Mock
	pongMock *mock.Mock
)

func setupTest() func(t *testing.T) {
	pingMock = new(mock.Mock)
	pongMock = new(mock.Mock)

	return func(t *testing.T) {
		pingMock.AssertExpectations(t)
		pingMock = nil
		pongMock.AssertExpectations(t)
		pongMock = nil
	}
}

func handle(s *Server, fromAddr string, fromID peer.ID, fromKey peer.PublicKey, data []byte) (bool, error) {
	msg, err := unmarshal(data)
	if err != nil {
		return false, err
	}

	switch msg.Type() {
	case MPing:
		pingMock.Called(s, fromAddr, fromID, fromKey, data)

	case MPong:
		if s.IsExpectedReply(fromAddr, fromID, MPong, msg) {
			pongMock.Called(s, fromAddr, fromID, fromKey, data)
		}

	default:
		panic("unknown message type")
	}

	return true, nil
}

func unmarshal(data []byte) (Message, error) {
	if len(data) != 1 {
		return nil, ErrInvalidMessage
	}

	switch MType(data[0]) {
	case MPing:
		return new(Ping), nil
	case MPong:
		return new(Pong), nil
	}
	return nil, ErrInvalidMessage
}

func TestSrvEncodeDecodePing(t *testing.T) {
	// create minimal server just containing the local peer
	local, err := peer.NewLocal("dummy", "local", peer.NewMemoryDB(log))
	require.NoError(t, err)
	s := &Server{local: local}

	ping := new(Ping)
	packet := s.encode(ping.Marshal())

	data, key, err := decode(packet)
	require.NoError(t, err)

	msg, _ := unmarshal(data)
	assert.Equal(t, local.ID(), key.ID())
	assert.Equal(t, msg, ping)
}

func newTestServer(t require.TestingT, name string, trans transport.Transport) (*Server, func()) {
	l := log.Named(name)
	db := peer.NewMemoryDB(l.Named("db"))
	local, err := peer.NewLocal(trans.LocalAddr().Network(), trans.LocalAddr().String(), db)
	require.NoError(t, err)

	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)

	srv := Listen(local, trans, l, HandlerFunc(handle))

	teardown := func() {
		srv.Close()
		db.Close()
	}
	return srv, teardown
}

func sendPing(s *Server, p *peer.Peer) error {
	ping := new(Ping)
	isPong := func(msg interface{}) bool {
		_, ok := msg.(*Pong)
		return ok
	}

	errc := s.SendExpectingReply(p.Address(), p.ID(), ping.Marshal(), MPong, isPong)
	return <-errc
}

func TestPingPong(t *testing.T) {
	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B)
	defer closeB()

	peerA := &srvA.Local().Peer
	peerB := &srvB.Local().Peer

	t.Run("A->B", func(t *testing.T) {
		defer setupTest()(t)

		// B expects valid ping from A and sends pong back
		pingMock.On("handle", srvB, peerA.Address(), peerA.ID(), peerA.PublicKey(), mock.Anything).Run(sendPong).Once()
		// A expects valid pong from B
		pongMock.On("handle", srvA, peerB.Address(), peerB.ID(), peerB.PublicKey(), mock.Anything).Once()

		assert.NoError(t, sendPing(srvA, peerB))
		time.Sleep(graceTime)

	})

	t.Run("B->A", func(t *testing.T) {
		defer setupTest()(t)

		pingMock.On("handle", srvA, peerB.Address(), peerB.ID(), peerB.PublicKey(), mock.Anything).Run(sendPong).Once() // A expects valid ping from B and sends pong back
		pongMock.On("handle", srvB, peerA.Address(), peerA.ID(), peerA.PublicKey(), mock.Anything).Once()               // B expects valid pong from A

		assert.NoError(t, sendPing(srvB, peerA))
		time.Sleep(graceTime)
	})
}

func TestSrvPingTimeout(t *testing.T) {
	defer setupTest()(t)

	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B)
	closeB()

	peerB := &srvB.Local().Peer
	assert.EqualError(t, sendPing(srvA, peerB), ErrTimeout.Error())
}

func TestUnexpectedPong(t *testing.T) {
	defer setupTest()(t)

	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B)
	defer closeB()

	// there should never be a Ping.Handle
	// there should never be a Pong.Handle

	srvA.Send(srvB.LocalAddr(), new(Pong).Marshal())
}
