package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	pb "github.com/iotaledger/hive.go/autopeering/server/proto"
	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/netutil"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrTimeout is returned when an expected incoming connection was not received in time.
	ErrTimeout = errors.New("accept timeout")
	// ErrDuplicateAccept is returned when the server already registered an accept request for that peer ID.
	ErrDuplicateAccept = errors.New("accept request for that peer already exists")
	// ErrClosed means that the server was shut down before a response could be received.
	ErrClosed = errors.New("server closed")
	// ErrInvalidHandshake is returned when no correct handshake could be established.
	ErrInvalidHandshake = errors.New("invalid handshake")
	// ErrNoGossip means that the given peer does not support the gossip service.
	ErrNoGossip = errors.New("peer does not have a gossip service")
)

// connection timeouts
const (
	defaultDialTimeout   = 1 * time.Second                    // timeout for net.Dial
	handshakeTimeout     = 500 * time.Millisecond             // read/write timeout of the handshake packages
	defaultAcceptTimeout = 3*time.Second + 2*handshakeTimeout // timeout after which the connection must be accepted.

	maxHandshakePacketSize = 256
)

// retry net.Dial once, on fail after 0.5s
var dialRetryPolicy = backoff.ConstantBackOff(500 * time.Millisecond).With(backoff.MaxRetries(1))

// TCP establishes verified incoming and outgoing TCP connections to other peers.
type TCP struct {
	local    *peer.Local
	listener *net.TCPListener
	log      *zap.SugaredLogger

	acceptReceivedCh chan accept
	matchersMap      map[identity.ID]*acceptMatcher
	matchersMutex    sync.RWMutex

	closeOnce sync.Once
	wg        sync.WaitGroup
	closing   chan struct{} // if this channel gets closed all pending waits should terminate
}

// connectResult contains the result of an incoming connection.
type connectResult struct {
	c   net.Conn
	err error
}

type acceptMatcher struct {
	peer      *peer.Peer // connecting peer
	connectCh chan connectResult
}

func newAcceptMatcher(p *peer.Peer) *acceptMatcher {
	return &acceptMatcher{peer: p, connectCh: make(chan connectResult, 1)}
}

type accept struct {
	fromID identity.ID // ID of the connecting peer
	req    []byte      // raw data of the handshake request
	conn   net.Conn    // the actual network connection
}

// ServeTCP creates the object and starts listening for incoming connections.
func ServeTCP(local *peer.Local, listener *net.TCPListener, log *zap.SugaredLogger) *TCP {
	t := &TCP{
		local:            local,
		listener:         listener,
		log:              log,
		acceptReceivedCh: make(chan accept),
		matchersMap:      map[identity.ID]*acceptMatcher{},
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

// ConnectPeerOption defines an option for the DialPeer and AcceptPeer methods.
type ConnectPeerOption func(conf *connectPeerConfig)

type connectPeerConfig struct {
	useDefaultTimeout bool
}

func buildConnectPeerConfig(opts []ConnectPeerOption) *connectPeerConfig {
	conf := &connectPeerConfig{
		useDefaultTimeout: true,
	}
	for _, o := range opts {
		o(conf)
	}
	return conf
}

// WithNoDefaultTimeout returns a ConnectPeerOption that disables the default timeout for dial or accept.
func WithNoDefaultTimeout() ConnectPeerOption {
	return func(conf *connectPeerConfig) {
		conf.useDefaultTimeout = false
	}
}

// DialPeer establishes a gossip connection to the given peer.
// If the peer does not accept the connection or the handshake fails, an error is returned.
func (t *TCP) DialPeer(ctx context.Context, p *peer.Peer, opts ...ConnectPeerOption) (net.Conn, error) {
	conf := buildConnectPeerConfig(opts)
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	var conn net.Conn
	if err := backoff.Retry(dialRetryPolicy, func() error {
		var err error
		address := net.JoinHostPort(p.IP().String(), strconv.Itoa(gossipEndpoint.Port()))
		dialer := &net.Dialer{}
		if conf.useDefaultTimeout {
			dialer.Timeout = defaultDialTimeout
		}
		conn, err = dialer.DialContext(ctx, "tcp", address)
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
func (t *TCP) AcceptPeer(ctx context.Context, p *peer.Peer, opts ...ConnectPeerOption) (net.Conn, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	// wait for the connection
	conn, err := t.acceptPeer(ctx, p, opts)
	if err != nil {
		return nil, fmt.Errorf("accept %s / %s failed: %w", net.JoinHostPort(p.IP().String(), strconv.Itoa(gossipEndpoint.Port())), p.ID(), err)
	}

	t.log.Debugw("incoming connection established",
		"id", p.ID(),
		"addr", conn.RemoteAddr(),
	)
	return conn, nil
}

func (t *TCP) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (net.Conn, error) {
	t.wg.Add(1)
	defer t.wg.Done()
	conf := buildConnectPeerConfig(opts)
	if conf.useDefaultTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultAcceptTimeout)
		defer cancel()
	}
	am := newAcceptMatcher(p)
	if ok := t.setAcceptMatcher(am); !ok {
		return nil, errors.WithStack(ErrDuplicateAccept)
	}
	defer t.removeAcceptMatcher(am)
	select {
	case connect := <-am.connectCh:
		return connect.c, errors.WithStack(connect.err)
	case <-ctx.Done():
		err := ctx.Err()
		if errors.Is(err, context.DeadlineExceeded) {
			t.log.Debugw("accept timeout", "id", am.peer.ID())
			return nil, errors.WithStack(ErrTimeout)
		}
		t.log.Debugw("context error", "id", am.peer.ID(), "err", err)
		return nil, errors.WithStack(err)
	case <-t.closing:
		return nil, errors.WithStack(ErrClosed)
	}
}

func (t *TCP) closeConnection(c net.Conn) {
	if err := c.Close(); err != nil {
		t.log.Warnw("close error", "err", err)
	}
}

func (t *TCP) run() {
	defer t.wg.Done()
	for {
		select {
		// on accept received, check all matchers for a fit
		case a := <-t.acceptReceivedCh:
			matched := t.matchNewConnection(a)
			// close the connection if not matched
			if !matched {
				t.log.Debugw("unexpected connection", "id", a.fromID, "addr", a.conn.RemoteAddr())
				t.closeConnection(a.conn)
			}
		case <-t.closing:
			return
		}
	}
}

func (t *TCP) matchNewConnection(a accept) bool {
	t.matchersMutex.RLock()
	defer t.matchersMutex.RUnlock()
	m, ok := t.matchersMap[a.fromID]
	if !ok {
		return false
	}
	// finish the handshake
	t.wg.Add(1)
	go t.matchAccept(m, a.req, a.conn)
	return true
}

func (t *TCP) setAcceptMatcher(m *acceptMatcher) bool {
	t.matchersMutex.Lock()
	defer t.matchersMutex.Unlock()
	_, exists := t.matchersMap[m.peer.ID()]
	if exists {
		return false
	}
	t.matchersMap[m.peer.ID()] = m
	return true
}

func (t *TCP) removeAcceptMatcher(m *acceptMatcher) {
	t.matchersMutex.Lock()
	defer t.matchersMutex.Unlock()
	delete(t.matchersMap, m.peer.ID())
}

func (t *TCP) matchAccept(m *acceptMatcher, req []byte, conn net.Conn) {
	defer t.wg.Done()

	if err := t.writeHandshakeResponse(req, conn); err != nil {
		m.connectCh <- connectResult{nil, fmt.Errorf("incoming handshake failed: %w", err)}

		t.closeConnection(conn)
		return
	}
	m.connectCh <- connectResult{conn, nil}
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
		case t.acceptReceivedCh <- accept{
			fromID: identity.NewID(key),
			req:    req,
			conn:   conn,
		}:
		case <-t.closing:
			t.closeConnection(conn)
			return
		}
	}
}

func (t *TCP) doHandshake(key ed25519.PublicKey, remoteAddr string, conn net.Conn) error {
	reqData, err := newHandshakeRequest(remoteAddr)
	if err != nil {
		return err
	}

	pkt := &pb.Packet{
		PublicKey: t.local.PublicKey().Bytes(),
		Signature: t.local.Sign(reqData).Bytes(),
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
	if err != nil || !bytes.Equal(key.Bytes(), signer.Bytes()) {
		return ErrInvalidHandshake
	}
	if !t.validateHandshakeResponse(pkt.GetData(), reqData) {
		return ErrInvalidHandshake
	}

	return nil
}

func (t *TCP) readHandshakeRequest(conn net.Conn) (ed25519.PublicKey, []byte, error) {
	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		return ed25519.PublicKey{}, nil, err
	}
	b := make([]byte, maxHandshakePacketSize)
	n, err := conn.Read(b)
	if err != nil {
		return ed25519.PublicKey{}, nil, fmt.Errorf("%w: %s", ErrInvalidHandshake, err.Error())
	}

	pkt := &pb.Packet{}
	err = proto.Unmarshal(b[:n], pkt)
	if err != nil {
		return ed25519.PublicKey{}, nil, err
	}

	key, err := peer.RecoverKeyFromSignedData(pkt)
	if err != nil {
		return ed25519.PublicKey{}, nil, err
	}

	if !t.validateHandshakeRequest(pkt.GetData()) {
		return ed25519.PublicKey{}, nil, ErrInvalidHandshake
	}

	return key, pkt.GetData(), nil
}

func (t *TCP) writeHandshakeResponse(reqData []byte, conn net.Conn) error {
	data, err := newHandshakeResponse(reqData)
	if err != nil {
		return err
	}

	pkt := &pb.Packet{
		PublicKey: t.local.PublicKey().Bytes(),
		Signature: t.local.Sign(data).Bytes(),
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
