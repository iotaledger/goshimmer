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
	p2pEndpoint := p.Services().Get(service.GossipKey)
	if p2pEndpoint == nil {
		return nil, ErrNoGossip
	}
	libp2pID, err := libp2putil.ToLibp2pPeerID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	addressStr := fmt.Sprintf("/ip4/%s/tcp/%d", p.IP(), p2pEndpoint.Port())
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
	for protocolID := range m.registeredProtocols {
		stream, err := m.initiateStream(ctx, libp2pID, protocolID)
		if err != nil {
			m.log.Errorf("dial %s / %s failed for proto %s: %w", address, p.ID(), protocolID, err)
		}
		m.log.Debugw("outgoing stream negotiated",
			"id", p.ID(),
			"addr", stream.Conn().RemoteMultiaddr(),
			"proto", protocolID,
		)
		streams[protocolID] = stream
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams initiated with peer %s / %s", address, p.ID())

	}

	return streams, nil
}

func (m *Manager) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (map[protocol.ID]*PacketsStream, error) {
	p2pEndpoint := p.Services().Get(service.GossipKey)
	if p2pEndpoint == nil {
		return nil, ErrNoGossip
	}

	handleInboundStream := func(protocolID protocol.ID) (*PacketsStream, error) {
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
			if ps.Protocol() != protocolID {
				return nil, fmt.Errorf("accepted stream has wrong protocol: %s != %s", ps.Protocol(), protocolID)
			}
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
	}

	var acceptWG sync.WaitGroup
	streams := make(map[protocol.ID]*PacketsStream)
	for protocolID := range m.registeredProtocols {
		acceptWG.Add(1)
		go func(protocolID protocol.ID) {
			defer acceptWG.Done()
			stream, err := handleInboundStream(protocolID)
			if err != nil {
				m.log.Errorf(
					"accept %s / %s proto %s failed: %w",
					net.JoinHostPort(p.IP().String(), strconv.Itoa(p2pEndpoint.Port())),
					p.ID(),
					protocolID,
					err,
				)
				return
			}
			m.log.Debugw("incoming stream negotiated",
				"id", p.ID(),
				"addr", stream.Conn().RemoteMultiaddr(),
				"proto", protocolID,
			)
			streams[protocolID] = stream
		}(protocolID)
	}
	acceptWG.Wait()

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams accepted from peer %s", p.ID())
	}

	return streams, nil
}

func (m *Manager) initiateStream(ctx context.Context, libp2pID libp2ppeer.ID, protocolID protocol.ID) (*PacketsStream, error) {
	protocolHandlers, registered := m.registeredProtocols[protocolID]
	if !registered {
		return nil, fmt.Errorf("cannot initiate stream protocol %s is not registered", protocolID)
	}
	stream, err := m.GetP2PHost().NewStream(ctx, libp2pID, protocolID)
	if err != nil {
		return nil, err
	}
	ps := NewPacketsStream(stream, protocolHandlers.PacketFactory)
	if err := protocolHandlers.NegotiationSend(ps); err != nil {
		err = errors.Wrap(err, "failed to send negotiation block")
		err = errors.CombineErrors(err, stream.Close())
		return nil, err
	}
	return ps, nil
}

func (m *Manager) handleStream(stream network.Stream) {
	protocolID := stream.Protocol()
	protocolHandlers, registered := m.registeredProtocols[protocolID]
	if !registered {
		m.log.Errorf("cannot accept stream: protocol %s is not registered", protocolID)
		m.CloseStream(stream)
	}
	ps := NewPacketsStream(stream, protocolHandlers.PacketFactory)
	if err := protocolHandlers.NegotiationReceive(ps); err != nil {
		m.log.Errorw("failed to receive negotiation message", "proto", protocolID, "err", err)
		m.CloseStream(stream)
	}
	am := m.MatchNewStream(stream)
	if am != nil {
		am.StreamCh <- ps
	} else {
		// close the connection if not matched
		m.log.Debugw("unexpected connection", "addr", stream.Conn().RemoteMultiaddr(),
			"id", stream.Conn().RemotePeer())
		m.CloseStream(stream)
	}
}

// AcceptMatcher holds data to match an existing connection with a peer.
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

// MatchNewStream matches a new stream with a peer.
func (m *Manager) MatchNewStream(stream network.Stream) *AcceptMatcher {
	m.acceptMutex.RLock()
	defer m.acceptMutex.RUnlock()
	am := m.acceptMap[stream.Conn().RemotePeer()]
	return am
}

// CloseStream closes a stream.
func (m *Manager) CloseStream(s network.Stream) {
	if err := s.Close(); err != nil {
		m.log.Warnw("close error", "err", err)
	}
}

// PacketsStream represents a stream of packets.
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

// NewPacketsStream creates a new PacketsStream.
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

// WritePacket writes a packet to the stream.
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

// ReadPacket reads a packet from the stream.
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
