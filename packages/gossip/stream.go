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
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

const (
	defaultConnectionTimeout = 5 * time.Second // timeout after which the connection must be established.
	protocolID               = "gossip/0.0.1"
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

func (m *Manager) dialPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (network.Stream, error) {
	conf := buildConnectPeerConfig(opts)
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}
	libp2pID, err := toLibp2pPeerID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	addressStr := fmt.Sprintf("/ip4%s/tcp/%d/p2p/%s", p.IP(), gossipEndpoint.Port(), libp2pID)
	address, err := multiaddr.NewMultiaddr(addressStr)
	if err != nil {
		return nil, err
	}
	m.host.Peerstore().AddAddr(libp2pID, address, peerstore.ConnectedAddrTTL)

	if conf.useDefaultTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultConnectionTimeout)
		defer cancel()
	}

	stream, err := m.host.NewStream(ctx, libp2pID, protocolID)
	if err != nil {
		return nil, errors.Wrapf(err, "dial %s / %s failed", address, p.ID())
	}
	m.log.Debugw("outgoing connection established",
		"id", p.ID(),
		"addr", stream.Conn().RemoteMultiaddr(),
	)
	return stream, nil
}

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
			ctx, cancel = context.WithTimeout(ctx, defaultConnectionTimeout)
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
	libp2pID, err := toLibp2pPeerID(p)
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

	am := m.matchNewStream(stream)
	if am != nil {
		am.streamCh <- stream
	} else {
		// close the connection if not matched
		m.log.Debugw("unexpected connection", "addr", stream.Conn().RemoteMultiaddr())
		m.closeStream(stream)
	}
}

func (m *Manager) matchNewStream(stream network.Stream) *acceptMatcher {
	m.acceptMutex.RLock()
	defer m.acceptMutex.RUnlock()
	am := m.acceptMap[stream.Conn().RemotePeer()]
	return am
}

func (m *Manager) closeStream(s network.Stream) {
	if err := s.Reset(); err != nil {
		m.log.Warnw("close error", "err", err)
	}
}
