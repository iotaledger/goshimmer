package gossip

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
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/atomic"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/libp2putil"
)

const (
	defaultConnectionTimeout = 5 * time.Second // timeout after which the connection must be established.
	protocolID               = "gossip/0.0.1"
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

func (m *Manager) dialPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (*packetsStream, error) {
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
	m.Libp2pHost.Peerstore().AddAddr(libp2pID, address, peerstore.ConnectedAddrTTL)

	if conf.useDefaultTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultConnectionTimeout)
		defer cancel()
	}

	stream, err := m.Libp2pHost.NewStream(ctx, libp2pID, protocolID)
	if err != nil {
		return nil, errors.Wrapf(err, "dial %s / %s failed", address, p.ID())
	}
	ps := newPacketsStream(stream)
	if err := sendNegotiationMessage(ps); err != nil {
		err = errors.Wrap(err, "failed to send negotiation message")
		err = errors.CombineErrors(err, stream.Close())
		return nil, err
	}
	m.log.Debugw("outgoing connection established",
		"id", p.ID(),
		"addr", stream.Conn().RemoteMultiaddr(),
	)
	return ps, nil
}

func (m *Manager) acceptPeer(ctx context.Context, p *peer.Peer, opts []ConnectPeerOption) (*packetsStream, error) {
	gossipEndpoint := p.Services().Get(service.GossipKey)
	if gossipEndpoint == nil {
		return nil, ErrNoGossip
	}

	ps, err := func() (*packetsStream, error) {
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
		case ps := <-am.streamCh:
			return ps, nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				m.log.Debugw("accept timeout", "id", am.peer.ID())
				return nil, errors.WithStack(ErrTimeout)
			}
			m.log.Debugw("context error", "id", am.peer.ID(), "err", err)
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
		"addr", ps.Conn().RemoteMultiaddr(),
	)
	return ps, nil
}

type acceptMatcher struct {
	peer     *peer.Peer // connecting peer
	libp2pID libp2ppeer.ID
	streamCh chan *packetsStream
}

func newAcceptMatcher(p *peer.Peer) (*acceptMatcher, error) {
	libp2pID, err := libp2putil.ToLibp2pPeerID(p)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &acceptMatcher{
		peer:     p,
		libp2pID: libp2pID,
		streamCh: make(chan *packetsStream, 1),
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
	ps := newPacketsStream(stream)
	if err := receiveNegotiationMessage(ps); err != nil {
		m.log.Warnw("Failed to receive negotiation message", "err", err)
		m.closeStream(stream)
		return
	}
	am := m.matchNewStream(stream)
	if am != nil {
		am.streamCh <- ps
	} else {
		// close the connection if not matched
		m.log.Debugw("unexpected connection", "addr", stream.Conn().RemoteMultiaddr(),
			"id", stream.Conn().RemotePeer())
		m.closeStream(stream)
	}
}

type packetsStream struct {
	network.Stream

	readerLock     sync.Mutex
	reader         *libp2putil.UvarintReader
	writerLock     sync.Mutex
	writer         *libp2putil.UvarintWriter
	packetsRead    *atomic.Uint64
	packetsWritten *atomic.Uint64
}

func newPacketsStream(stream network.Stream) *packetsStream {
	return &packetsStream{
		Stream:         stream,
		reader:         libp2putil.NewDelimitedReader(stream),
		writer:         libp2putil.NewDelimitedWriter(stream),
		packetsRead:    atomic.NewUint64(0),
		packetsWritten: atomic.NewUint64(0),
	}
}

func (ps *packetsStream) writePacket(packet *pb.Packet) error {
	ps.writerLock.Lock()
	defer ps.writerLock.Unlock()
	if err := ps.SetWriteDeadline(time.Now().Add(ioTimeout)); err != nil && !isDeadlineUnsupportedError(err) {
		return errors.WithStack(err)
	}
	err := ps.writer.WriteMsg(packet)
	if err != nil {
		return errors.WithStack(err)
	}
	ps.packetsWritten.Inc()
	return nil
}

func (ps *packetsStream) readPacket(packet *pb.Packet) error {
	ps.readerLock.Lock()
	defer ps.readerLock.Unlock()
	if err := ps.SetReadDeadline(time.Now().Add(ioTimeout)); err != nil && !isDeadlineUnsupportedError(err) {
		return errors.WithStack(err)
	}
	if err := ps.reader.ReadMsg(packet); err != nil {
		return errors.WithStack(err)
	}
	ps.packetsRead.Inc()
	return nil
}

func sendNegotiationMessage(ps *packetsStream) error {
	packet := &pb.Packet{Body: &pb.Packet_Negotiation{Negotiation: &pb.Negotiation{}}}
	return errors.WithStack(ps.writePacket(packet))
}

func receiveNegotiationMessage(ps *packetsStream) (err error) {
	packet := &pb.Packet{}
	if err := ps.readPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*pb.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}

func (m *Manager) matchNewStream(stream network.Stream) *acceptMatcher {
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

func isDeadlineUnsupportedError(err error) bool {
	return strings.Contains(err.Error(), "deadline not supported")
}

func isTimeoutError(err error) bool {
	return os.IsTimeout(errors.Unwrap(err))
}
