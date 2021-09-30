package gossip

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/libp2p/go-libp2p-core/network"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
)

const (
	defaultDialTimeout   = 1 * time.Second                    // timeout for net.Dial
	handshakeTimeout     = 500 * time.Millisecond             // read/write timeout of the handshake packages
	defaultAcceptTimeout = 3*time.Second + 2*handshakeTimeout // timeout after which the connection must be accepted.
	protocolID           = "gossip/0.0.1"
)

var (
	// ErrTimeout is returned when an expected incoming connection was not received in time.
	ErrTimeout = errors.New("accept timeout")
	// ErrDuplicateAccept is returned when the server already registered an accept request for that peer ID.
	ErrDuplicateAccept = errors.New("accept request for that peer already exists")
	// ErrClosed means that the server was shut down before a response could be received.
	ErrClosed = errors.New("server closed")
	// ErrNoGossip means that the given peer does not support the gossip service.
	ErrNoGossip = errors.New("peer does not have a gossip service")
)

func (m *Manager) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (network.Stream, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	stream, err := func() (network.Stream, error) {
		// wait for the connection
		m.acceptWG.Add(1)
		defer m.acceptWG.Done()
		conf := buildConnectPeerConfig(opts)
		if conf.useDefaultTimeout {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultAcceptTimeout)
			defer cancel()
		}
		am, err := newAcceptMatcher(p)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if ok := m.setAcceptMatcher(am); !ok {
			return nil, errors.WithStack(ErrDuplicateAccept)
		}
		defer m.removeAcceptMatcher(am)
		select {
		case stream := <-am.streamCh:
			return stream, nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				m.log.Debugw("accept timeout", "id", am.peer.ID())
				return nil, errors.WithStack(ErrTimeout)
			}
			m.log.Debugw("context error", "id", am.peer.ID(), "err", err)
			return nil, errors.WithStack(err)
		case <-m.closing:
			return nil, errors.WithStack(ErrClosed)
		}

	}()

	if err != nil {
		return nil, fmt.Errorf(
			"accept %s / %s failed: %w",
			net.JoinHostPort(p.IP().String(), strconv.Itoa(gossipEndpoint.Port())), p.ID(),
			err,
		)
	}
	m.log.Debugw("incoming connection established",
		"id", p.ID(),
		"addr", stream.Conn().RemoteMultiaddr(),
	)
	return stream, nil

}

type acceptMatcher struct {
	peer     *peer.Peer // connecting peer
	libp2pID libp2ppeer.ID
	streamCh chan network.Stream
}

func newAcceptMatcher(p *peer.Peer) (*acceptMatcher, error) {
	libp2pID, err := toLibp2pID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &acceptMatcher{
		peer:     p,
		libp2pID: libp2pID,
		streamCh: make(chan network.Stream, 1),
	}, nil
}

func (m *Manager) setAcceptMatcher(am *acceptMatcher) bool {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()
	_, exists := m.acceptMap[am.libp2pID]
	if exists {
		return false
	}
	m.acceptMap[am.libp2pID] = am
	return true
}

func (m *Manager) removeAcceptMatcher(am *acceptMatcher) {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()
	delete(m.acceptMap, am.libp2pID)
}

func (m *Manager) streamHandler(stream network.Stream) {

	matched := m.matchNewStream(stream)
	// close the connection if not matched
	if !matched {
		m.log.Debugw("unexpected connection", "libp2pAddr", stream.Conn().RemoteMultiaddr())
		m.closeStream(stream)
	}
}

func (m *Manager) matchNewStream(stream network.Stream) bool {
	m.acceptMutex.RLock()
	defer m.acceptMutex.RUnlock()
	am, ok := m.acceptMap[stream.Conn().RemotePeer()]
	if !ok {
		return false
	}
	am.streamCh <- stream
	return true
}

func (m *Manager) closeStream(s network.Stream) {
	if err := s.Reset(); err != nil {
		m.log.Warnw("close error", "err", err)
	}
}
