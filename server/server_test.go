package server

import (
	"log"
	"testing"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/salt"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	s := args.Get(0).(*Server)
	p := args.Get(1).(*peer.Peer)
	s.Send(p.Address(), new(Pong).Marshal())
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

func handle(s *Server, from *peer.Peer, data []byte) (bool, error) {
	msg, err := unmarshal(data)
	if err != nil {
		return false, err
	}

	switch msg.Type() {
	case MPing:
		pingMock.Called(s, from)

	case MPong:
		if s.IsExpectedReply(from, MPong, msg) {
			pongMock.Called(s, from)
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
	local, err := peer.NewLocal(peer.NewMemoryDB(logger))
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

func newTestServer(t require.TestingT, name string, trans transport.Transport, logger *zap.SugaredLogger) (*Server, func()) {
	log := logger.Named(name)
	db := peer.NewMemoryDB(log.Named("db"))
	local, err := peer.NewLocal(db)
	require.NoError(t, err)

	s, _ := salt.NewSalt(100 * time.Second)
	local.SetPrivateSalt(s)
	s, _ = salt.NewSalt(100 * time.Second)
	local.SetPublicSalt(s)

	srv := Listen(local, trans, logger.Named(name), HandlerFunc(handle))

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

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	peerA := peer.NewPeer(srvA.Local().PublicKey(), srvA.LocalAddr())
	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	t.Run("A->B", func(t *testing.T) {
		defer setupTest()(t)

		pingMock.On("handle", srvB, peerA).Run(sendPong).Once() // B expects valid ping from A and sends pong back
		pongMock.On("handle", srvA, peerB).Once()               // A expects valid pong from B

		assert.NoError(t, sendPing(srvA, peerB))
		time.Sleep(graceTime)

	})

	t.Run("B->A", func(t *testing.T) {
		defer setupTest()(t)

		pingMock.On("handle", srvA, peerB).Run(sendPong).Once() // A expects valid ping from B and sends pong back
		pongMock.On("handle", srvB, peerA).Once()               // B expects valid pong from A

		assert.NoError(t, sendPing(srvB, peerA))
		time.Sleep(graceTime)
	})
}

func TestSrvPingTimeout(t *testing.T) {
	defer setupTest()(t)

	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	closeB()

	peerB := peer.NewPeer(srvB.Local().PublicKey(), srvB.LocalAddr())

	assert.EqualError(t, sendPing(srvA, peerB), ErrTimeout.Error())
}

func TestUnexpectedPong(t *testing.T) {
	defer setupTest()(t)

	p2p := transport.P2P()

	srvA, closeA := newTestServer(t, "A", p2p.A, logger)
	defer closeA()
	srvB, closeB := newTestServer(t, "B", p2p.B, logger)
	defer closeB()

	// there should never be a Ping.Handle
	// there should never be a Pong.Handle

	srvA.Send(srvB.LocalAddr(), new(Pong).Marshal())
}
