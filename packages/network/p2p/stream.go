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

	"github.com/libp2p/go-libp2p/core/network"
	libp2ppeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/libp2putil"
	pp "github.com/iotaledger/goshimmer/packages/network/p2p/proto"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
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
	// ErrNoP2P means that the given peer does not support the p2p service.
	ErrNoP2P = errors.New("peer does not have a p2p service")
)

func (m *Manager) dialPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (map[protocol.ID]*PacketsStream, error) {
	m.registeredProtocolsMutex.RLock()
	defer m.registeredProtocolsMutex.RUnlock()

	conf := buildConnectPeerConfig(opts)
	p2pEndpoint := p.Services().Get(service.P2PKey)
	if p2pEndpoint == nil {
		return nil, ErrNoP2P
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
			continue
		}
		m.log.Debugw("outgoing stream negotiated",
			"id", p.ID(),
			"addr", stream.Conn().RemoteMultiaddr(),
			"proto", protocolID,
		)
		streams[protocolID] = stream
	}

	if len(streams) == 0 {
		return nil, errors.Errorf("no streams initiated with peer %s / %s", address, p.ID())
	}

	return streams, nil
}

func (m *Manager) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (map[protocol.ID]*PacketsStream, error) {
	m.registeredProtocolsMutex.RLock()
	defer m.registeredProtocolsMutex.RUnlock()

	p2pEndpoint := p.Services().Get(service.P2PKey)
	if p2pEndpoint == nil {
		return nil, ErrNoP2P
	}

	handleInboundStream := func(ctx context.Context, protocolID protocol.ID) (*PacketsStream, error) {
		if buildConnectPeerConfig(opts).useDefaultTimeout {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultConnectionTimeout)
			defer cancel()
		}
		amCtx, am, err := m.newAcceptMatcher(ctx, p, protocolID)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if am == nil {
			return nil, errors.WithStack(ErrDuplicateAccept)
		}
		defer m.removeAcceptMatcher(am, protocolID)

		m.log.Debugw("waiting for incoming stream", "id", am.Peer.ID(), "proto", protocolID)
		am.StreamChMutex.RLock()
		streamCh := am.StreamCh[protocolID]
		am.StreamChMutex.RUnlock()
		select {
		case ps := <-streamCh:
			if ps.Protocol() != protocolID {
				return nil, errors.Errorf("accepted stream has wrong protocol: %s != %s", ps.Protocol(), protocolID)
			}
			return ps, nil
		case <-amCtx.Done():
			err := amCtx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				m.log.Debugw("accept timeout", "id", am.Peer.ID(), "proto", protocolID)
				return nil, errors.WithStack(ErrTimeout)
			}
			m.log.Debugw("context error", "id", am.Peer.ID(), "err", err)
			return nil, errors.WithStack(err)
		}
	}

	var acceptWG sync.WaitGroup
	streamsChan := make(chan *PacketsStream, len(m.registeredProtocols))
	for protocolID := range m.registeredProtocols {
		acceptWG.Add(1)
		go func(protocolID protocol.ID) {
			defer acceptWG.Done()
			stream, err := handleInboundStream(ctx, protocolID)
			if err != nil {
				m.log.Errorf(
					"accept %s / %s proto %s failed: %s",
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
			streamsChan <- stream
		}(protocolID)
	}
	acceptWG.Wait()
	close(streamsChan)

	streams := make(map[protocol.ID]*PacketsStream)
	for stream := range streamsChan {
		streams[stream.Protocol()] = stream
	}

	if len(streams) == 0 {
		return nil, errors.Errorf("no streams accepted from peer %s", p.ID())
	}

	return streams, nil
}

func (m *Manager) initiateStream(ctx context.Context, libp2pID libp2ppeer.ID, protocolID protocol.ID) (*PacketsStream, error) {
	protocolHandler, registered := m.registeredProtocols[protocolID]
	if !registered {
		return nil, errors.Errorf("cannot initiate stream protocol %s is not registered", protocolID)
	}
	stream, err := m.GetP2PHost().NewStream(ctx, libp2pID, protocolID)
	if err != nil {
		return nil, err
	}
	ps := NewPacketsStream(stream, protocolHandler.PacketFactory)
	if err := ps.sendNegotiation(); err != nil {
		err = errors.Wrap(err, "failed to send negotiation block")
		stream.Close()
		return nil, err
	}
	return ps, nil
}

func (m *Manager) handleStream(stream network.Stream) {
	m.registeredProtocolsMutex.RLock()
	defer m.registeredProtocolsMutex.RUnlock()

	protocolID := stream.Protocol()
	protocolHandler, registered := m.registeredProtocols[protocolID]
	if !registered {
		m.log.Errorf("cannot accept stream: protocol %s is not registered", protocolID)
		m.closeStream(stream)
		return
	}
	ps := NewPacketsStream(stream, protocolHandler.PacketFactory)
	if err := ps.receiveNegotiation(); err != nil {
		m.log.Errorw("failed to receive negotiation message", "proto", protocolID, "err", err)
		m.closeStream(stream)
		return
	}
	am := m.matchNewStream(stream)
	if am != nil {
		am.StreamChMutex.RLock()
		defer am.StreamChMutex.RUnlock()
		streamCh := am.StreamCh[protocolID]

		select {
		case <-am.Ctx.Done():
		case streamCh <- ps:
			m.log.Debugw("incoming stream matched", "id", am.Peer.ID(), "proto", protocolID)
		}
	} else {
		// close the connection if not matched
		m.log.Debugw("unexpected connection", "addr", stream.Conn().RemoteMultiaddr(),
			"id", stream.Conn().RemotePeer(), "proto", protocolID)
		m.closeStream(stream)
		stream.Conn().Close()
	}
}

// AcceptMatcher holds data to match an existing connection with a peer.
type AcceptMatcher struct {
	Peer          *peer.Peer // connecting peer
	Libp2pID      libp2ppeer.ID
	StreamChMutex sync.RWMutex
	StreamCh      map[protocol.ID]chan *PacketsStream
	Ctx           context.Context
	CtxCancel     context.CancelFunc
}

func (m *Manager) newAcceptMatcher(ctx context.Context, p *peer.Peer, protocolID protocol.ID) (context.Context, *AcceptMatcher, error) {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()

	libp2pID, err := libp2putil.ToLibp2pPeerID(p)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	acceptMatcher, acceptExists := m.acceptMap[libp2pID]
	if acceptExists {
		acceptMatcher.StreamChMutex.Lock()
		defer acceptMatcher.StreamChMutex.Unlock()
		if _, streamChanExists := acceptMatcher.StreamCh[protocolID]; streamChanExists {
			return nil, nil, nil
		}
		acceptMatcher.StreamCh[protocolID] = make(chan *PacketsStream)
		return acceptMatcher.Ctx, acceptMatcher, nil
	}

	cancelCtx, cancelCtxFunc := context.WithCancel(ctx)

	am := &AcceptMatcher{
		Peer:      p,
		Libp2pID:  libp2pID,
		StreamCh:  make(map[protocol.ID]chan *PacketsStream),
		Ctx:       cancelCtx,
		CtxCancel: cancelCtxFunc,
	}

	am.StreamCh[protocolID] = make(chan *PacketsStream)

	m.acceptMap[libp2pID] = am

	return cancelCtx, am, nil
}

func (m *Manager) removeAcceptMatcher(am *AcceptMatcher, protocolID protocol.ID) {
	m.acceptMutex.Lock()
	defer m.acceptMutex.Unlock()

	existingAm := m.acceptMap[am.Libp2pID]

	existingAm.StreamChMutex.Lock()
	defer existingAm.StreamChMutex.Unlock()

	delete(existingAm.StreamCh, protocolID)

	if len(existingAm.StreamCh) == 0 {
		delete(m.acceptMap, am.Libp2pID)
		existingAm.CtxCancel()
	}
}

func (m *Manager) matchNewStream(stream network.Stream) *AcceptMatcher {
	m.acceptMutex.RLock()
	defer m.acceptMutex.RUnlock()
	am := m.acceptMap[stream.Conn().RemotePeer()]
	return am
}

func (m *Manager) closeStream(s network.Stream) {
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
	if err := ps.reader.ReadBlk(message); err != nil {
		return errors.WithStack(err)
	}
	ps.packetsRead.Inc()
	return nil
}

func (ps *PacketsStream) sendNegotiation() error {
	return errors.WithStack(ps.WritePacket(&pp.Negotiation{}))
}

func (ps *PacketsStream) receiveNegotiation() (err error) {
	return errors.WithStack(ps.ReadPacket(&pp.Negotiation{}))
}

func isDeadlineUnsupportedError(err error) bool {
	return strings.Contains(err.Error(), "deadline not supported")
}

func isTimeoutError(err error) bool {
	return os.IsTimeout(errors.Unwrap(err))
}
