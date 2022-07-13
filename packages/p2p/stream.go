package p2p

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/libp2p/go-libp2p-core/network"
	libp2ppeer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/libp2putil"
)

const (
	defaultConnectionTimeout = 5 * time.Second // timeout after which the connection must be established.
	ioTimeout                = 4 * time.Second
)

var (
	// ErrTimeout is returned when an expected incoming connection was not received in time.
	ErrTimeout = errors.New("accept timeout")
	// ErrDuplicateAccept is returned when the server already registered an accept request for that peer ID.
	ErrDuplicateAccept = errors.New("accept request for that peer already exists")
	// ErrNoGossip means that the given peer does not support the gossip service.
	ErrNoGossip = errors.New("peer does not have a gossip service")
)

func (m *Manager) dialPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (map[protocol.ID]*PacketsStream, error) {
	conf := buildConnectPeerConfig(opts)
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}
	libp2pID, err := libp2putil.ToLibp2pPeerID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	addressStr := fmt.Sprintf("/ip4/%s/tcp/%d", p.IP(), gossipEndpoint.Port())
	address, err := multiaddr.NewMultiaddr(addressStr)
	if err != nil {
		return nil, err
	}
	m.libp2pHost.Peerstore().AddAddr(libp2pID, address, peerstore.ConnectedAddrTTL)

	if conf.useDefaultTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultConnectionTimeout)
		defer cancel()
	}

	streams := make(map[protocol.ID]*PacketsStream)
	for protocolID, handlers := range m.protocols {
		stream, err := handlers.StreamEstablishFunc(ctx, libp2pID)
		if err != nil {
			return nil, errors.Wrapf(err, "dial %s / %s failed", address, p.ID())
		}
		m.log.Debugw("outgoing connection established",
			"id", p.ID(),
			"addr", stream.Conn().RemoteMultiaddr(),
			"proto", protocolID,
		)
		streams[protocolID] = stream
	}

	return streams, err
}

func (m *Manager) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (map[protocol.ID]*PacketsStream, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	stream, err := func() (*PacketsStream, error) {
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
		case ps := <-am.StreamCh:
			return ps, nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				m.log.Debugw("accept timeout", "id", am.Peer.ID())
				return nil, errors.WithStack(ErrTimeout)
			}
			m.log.Debugw("context error", "id", am.Peer.ID(), "err", err)
			return nil, errors.WithStack(err)
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
	return map[protocol.ID]*PacketsStream{stream.Protocol(): stream}, nil
}

type AcceptMatcher struct {
	Peer     *peer.Peer // connecting peer
	Libp2pID libp2ppeer.ID
	StreamCh chan *PacketsStream
}

func newAcceptMatcher(p *peer.Peer) (*AcceptMatcher, error) {
	libp2pID, err := libp2putil.ToLibp2pPeerID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &AcceptMatcher{
		Peer:     p,
		Libp2pID: libp2pID,
		StreamCh: make(chan *PacketsStream, 1),
	}, nil
}

func (m *Manager) setAcceptMatcher(am *AcceptMatcher) bool {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()
	_, exists := m.acceptMap[am.Libp2pID]
	if exists {
		return false
	}
	m.acceptMap[am.Libp2pID] = am
	return true
}

func (m *Manager) removeAcceptMatcher(am *AcceptMatcher) {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()
	delete(m.acceptMap, am.Libp2pID)
}
func (m *Manager) MatchNewStream(stream network.Stream) *AcceptMatcher {
	m.acceptMutex.RLock()
	defer m.acceptMutex.RUnlock()
	am := m.acceptMap[stream.Conn().RemotePeer()]
	return am
}

func (m *Manager) CloseStream(s network.Stream) {
	if err := s.Close(); err != nil {
		m.log.Warnw("close error", "err", err)
	}
}

type PacketsStream struct {
	network.Stream
	packetFactory func() proto.Message

	readerLock     sync.Mutex
	reader         *libp2putil.UvarintReader
	writerLock     sync.Mutex
	writer         *libp2putil.UvarintWriter
	packetsRead    *atomic.Uint64
	packetsWritten *atomic.Uint64
}

func NewPacketsStream(stream network.Stream, packetFactory func() proto.Message) *PacketsStream {
	return &PacketsStream{
		Stream:         stream,
		packetFactory:  packetFactory,
		reader:         libp2putil.NewDelimitedReader(stream),
		writer:         libp2putil.NewDelimitedWriter(stream),
		packetsRead:    atomic.NewUint64(0),
		packetsWritten: atomic.NewUint64(0),
	}
}

func (ps *PacketsStream) WritePacket(message proto.Message) error {
	ps.writerLock.Lock()
	defer ps.writerLock.Unlock()
	if err := ps.SetWriteDeadline(time.Now().Add(ioTimeout)); err != nil && !isDeadlineUnsupportedError(err) {
		return errors.WithStack(err)
	}
	err := ps.writer.WriteBlk(message)
	if err != nil {
		return errors.WithStack(err)
	}
	ps.packetsWritten.Inc()
	return nil
}

func (ps *PacketsStream) ReadPacket(message proto.Message) error {
	ps.readerLock.Lock()
	defer ps.readerLock.Unlock()
	if err := ps.SetReadDeadline(time.Now().Add(ioTimeout)); err != nil && !isDeadlineUnsupportedError(err) {
		return errors.WithStack(err)
	}
	if err := ps.reader.ReadBlk(message); err != nil {
		return errors.WithStack(err)
	}
	ps.packetsRead.Inc()
	return nil
}

func isDeadlineUnsupportedError(err error) bool {
	return strings.Contains(err.Error(), "deadline not supported")
}

func isTimeoutError(err error) bool {
	return os.IsTimeout(errors.Unwrap(err))
}
