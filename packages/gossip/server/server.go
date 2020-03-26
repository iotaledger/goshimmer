package server

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	pb "github.com/iotaledger/hive.go/autopeering/server/proto"
	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/netutil"
	"go.uber.org/zap"
)

var (
	// ErrTimeout is returned when an expected incoming connection was not received in time.
	ErrTimeout = errors.New("accept timeout")
	// ErrClosed means that the server was shut down before a response could be received.
	ErrClosed = errors.New("server closed")
	// ErrInvalidHandshake is returned when no correct handshake could be established.
	ErrInvalidHandshake = errors.New("invalid handshake")
	// ErrNoGossip means that the given peer does not support the gossip service.
	ErrNoGossip = errors.New("peer does not have a gossip service")
)

// connection timeouts
const (
	dialTimeout       = 1 * time.Second                    // timeout for net.Dial
	handshakeTimeout  = 500 * time.Millisecond             // read/write timeout of the handshake packages
	acceptTimeout     = 3 * time.Second                    // timeout to accept incoming connections
	connectionTimeout = acceptTimeout + 2*handshakeTimeout // timeout after which the connection must be established

	maxHandshakePacketSize = 256
)

// retry net.Dial once, on fail after 0.5s
var dialRetryPolicy = backoff.ConstantBackOff(500 * time.Millisecond).With(backoff.MaxRetries(1))

// TCP establishes verified incoming and outgoing TCP connections to other peers.
type TCP struct {
	local    *peer.Local
	listener *net.TCPListener
	log      *zap.SugaredLogger

	addAcceptMatcher chan *acceptMatcher
	acceptReceived   chan accept

	closeOnce sync.Once
	wg        sync.WaitGroup
	closing   chan struct{} // if this channel gets closed all pending waits should terminate
}

// connect contains the result of an incoming connection.
type connect struct {
	c   net.Conn
	err error
}

type acceptMatcher struct {
	peer      *peer.Peer   // connecting peer
	deadline  time.Time    // deadline for the incoming call
	connected chan connect // result of the connection is signaled here
}

type accept struct {
	fromID peer.ID  // ID of the connecting peer
	req    []byte   // raw data of the handshake request
	conn   net.Conn // the actual network connection
}

// ServeTCP creates the object and starts listening for incoming connections.
func ServeTCP(local *peer.Local, listener *net.TCPListener, log *zap.SugaredLogger) *TCP {
	t := &TCP{
		local:            local,
		listener:         listener,
		log:              log,
		addAcceptMatcher: make(chan *acceptMatcher),
		acceptReceived:   make(chan accept),
		closing:          make(chan struct{}),
	}

	t.log.Debugw("server started",
		"network", listener.Addr().Network(),
		"address", listener.Addr().String(),
	)

	t.wg.Add(2)
	go t.run()
	go t.listenLoop()

	return t
}

// Close stops listening on the gossip address.
func (t *TCP) Close() {
	t.closeOnce.Do(func() {
		close(t.closing)
		if err := t.listener.Close(); err != nil {
			t.log.Warnw("close error", "err", err)
		}
		t.wg.Wait()
	})
}

// LocalAddr returns the listener's network address,
func (t *TCP) LocalAddr() net.Addr {
	return t.listener.Addr()
}

// DialPeer establishes a gossip connection to the given peer.
// If the peer does not accept the connection or the handshake fails, an error is returned.
func (t *TCP) DialPeer(p *peer.Peer) (net.Conn, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	var conn net.Conn
	if err := backoff.Retry(dialRetryPolicy, func() error {
		var err error
		address := net.JoinHostPort(p.IP().String(), strconv.Itoa(gossipEndpoint.Port()))
		conn, err = net.DialTimeout("tcp", address, dialTimeout)
		if err != nil {
			return fmt.Errorf("dial %s / %s failed: %w", address, p.ID(), err)
		}

		if err = t.doHandshake(p.PublicKey(), address, conn); err != nil {
			return fmt.Errorf("handshake %s / %s failed: %w", address, p.ID(), err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	t.log.Debugw("outgoing connection established",
		"id", p.ID(),
		"addr", conn.RemoteAddr(),
	)
	return conn, nil
}

// AcceptPeer awaits an incoming connection from the given peer.
// If the peer does not establish the connection or the handshake fails, an error is returned.
func (t *TCP) AcceptPeer(p *peer.Peer) (net.Conn, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	// wait for the connection
	connected := <-t.acceptPeer(p)
	if connected.err != nil {
		return nil, fmt.Errorf("accept %s / %s failed: %w", net.JoinHostPort(p.IP().String(), strconv.Itoa(gossipEndpoint.Port())), p.ID(), connected.err)
	}

	t.log.Debugw("incoming connection established",
		"id", p.ID(),
		"addr", connected.c.RemoteAddr(),
	)
	return connected.c, nil
}

func (t *TCP) acceptPeer(p *peer.Peer) <-chan connect {
	connected := make(chan connect, 1)
	// add the matcher
	select {
	case t.addAcceptMatcher <- &acceptMatcher{peer: p, connected: connected}:
	case <-t.closing:
		connected <- connect{nil, ErrClosed}
	}
	return connected
}

func (t *TCP) closeConnection(c net.Conn) {
	if err := c.Close(); err != nil {
		t.log.Warnw("close error", "err", err)
	}
}

func (t *TCP) run() {
	defer t.wg.Done()

	var (
		matcherList = list.New()
		timeout     = time.NewTimer(0)
	)
	defer timeout.Stop()

	<-timeout.C // ignore first timeout

	for {

		// Set the timer so that it fires when the next accept expires
		if e := matcherList.Front(); e != nil {
			// the first element always has the closest deadline
			m := e.Value.(*acceptMatcher)
			timeout.Reset(time.Until(m.deadline))
		} else {
			timeout.Stop()
		}

		select {

		// add a new matcher to the list
		case m := <-t.addAcceptMatcher:
			m.deadline = time.Now().Add(connectionTimeout)
			matcherList.PushBack(m)

		// on accept received, check all matchers for a fit
		case a := <-t.acceptReceived:
			matched := false
			for e := matcherList.Front(); e != nil; e = e.Next() {
				m := e.Value.(*acceptMatcher)
				if m.peer.ID() == a.fromID {
					matched = true
					matcherList.Remove(e)
					// finish the handshake
					go t.matchAccept(m, a.req, a.conn)
				}
			}
			// close the connection if not matched
			if !matched {
				t.log.Debugw("unexpected connection", "id", a.fromID, "addr", a.conn.RemoteAddr())
				t.closeConnection(a.conn)
			}

		// on timeout, check for expired matchers
		case <-timeout.C:
			now := time.Now()

			// notify and remove any expired matchers
			for e := matcherList.Front(); e != nil; e = e.Next() {
				m := e.Value.(*acceptMatcher)
				if now.After(m.deadline) || now.Equal(m.deadline) {
					t.log.Debugw("accept timeout", "id", m.peer.ID())
					m.connected <- connect{nil, ErrTimeout}
					matcherList.Remove(e)
				}
			}

		// on close, notify all the matchers
		case <-t.closing:
			for e := matcherList.Front(); e != nil; e = e.Next() {
				e.Value.(*acceptMatcher).connected <- connect{nil, ErrClosed}
			}
			return

		}
	}
}

func (t *TCP) matchAccept(m *acceptMatcher, req []byte, conn net.Conn) {
	t.wg.Add(1)
	defer t.wg.Done()

	if err := t.writeHandshakeResponse(req, conn); err != nil {
		m.connected <- connect{nil, fmt.Errorf("incoming handshake failed: %w", err)}
		t.closeConnection(conn)
		return
	}
	m.connected <- connect{conn, nil}
}

func (t *TCP) listenLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.AcceptTCP()
		if netutil.IsTemporaryError(err) {
			t.log.Debugw("temporary read error", "err", err)
			continue
		}
		if err != nil {
			// return from the loop on all other errors
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				t.log.Warnw("listen error", "err", err)
			}
			t.log.Debug("listening stopped")
			return
		}

		key, req, err := t.readHandshakeRequest(conn)
		if err != nil {
			t.log.Warnw("failed handshake", "addr", conn.RemoteAddr(), "err", err)
			t.closeConnection(conn)
			continue
		}

		select {
		case t.acceptReceived <- accept{
			fromID: key.ID(),
			req:    req,
			conn:   conn,
		}:
		case <-t.closing:
			t.closeConnection(conn)
			return
		}
	}
}

func (t *TCP) doHandshake(key peer.PublicKey, remoteAddr string, conn net.Conn) error {
	reqData, err := newHandshakeRequest(remoteAddr)
	if err != nil {
		return err
	}

	pkt := &pb.Packet{
		PublicKey: t.local.PublicKey(),
		Signature: t.local.Sign(reqData),
		Data:      reqData,
	}
	b, err := proto.Marshal(pkt)
	if err != nil {
		return err
	}
	if l := len(b); l > maxHandshakePacketSize {
		return fmt.Errorf("handshake size too large: %d, max %d", l, maxHandshakePacketSize)
	}

	err = conn.SetWriteDeadline(time.Now().Add(handshakeTimeout))
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	err = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	if err != nil {
		return err
	}
	b = make([]byte, maxHandshakePacketSize)
	n, err := conn.Read(b)
	if err != nil {
		return err
	}

	pkt = &pb.Packet{}
	err = proto.Unmarshal(b[:n], pkt)
	if err != nil {
		return err
	}

	signer, err := peer.RecoverKeyFromSignedData(pkt)
	if err != nil || !bytes.Equal(key, signer) {
		return ErrInvalidHandshake
	}
	if !t.validateHandshakeResponse(pkt.GetData(), reqData) {
		return ErrInvalidHandshake
	}

	return nil
}

func (t *TCP) readHandshakeRequest(conn net.Conn) (peer.PublicKey, []byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return nil, nil, err
	}
	b := make([]byte, maxHandshakePacketSize)
	n, err := conn.Read(b)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", ErrInvalidHandshake, err.Error())
	}

	pkt := &pb.Packet{}
	err = proto.Unmarshal(b[:n], pkt)
	if err != nil {
		return nil, nil, err
	}

	key, err := peer.RecoverKeyFromSignedData(pkt)
	if err != nil {
		return nil, nil, err
	}

	if !t.validateHandshakeRequest(pkt.GetData()) {
		return nil, nil, ErrInvalidHandshake
	}

	return key, pkt.GetData(), nil
}

func (t *TCP) writeHandshakeResponse(reqData []byte, conn net.Conn) error {
	data, err := newHandshakeResponse(reqData)
	if err != nil {
		return err
	}

	pkt := &pb.Packet{
		PublicKey: t.local.PublicKey(),
		Signature: t.local.Sign(data),
		Data:      data,
	}
	b, err := proto.Marshal(pkt)
	if err != nil {
		return err
	}
	if l := len(b); l > maxHandshakePacketSize {
		return fmt.Errorf("handshake size too large: %d, max %d", l, maxHandshakePacketSize)
	}

	err = conn.SetWriteDeadline(time.Now().Add(handshakeTimeout))
	if err != nil {
		return err
	}
	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	return nil
}
