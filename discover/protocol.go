package discover

import (
	"bytes"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	pb "github.com/wollac/autopeering/discover/proto"
	"github.com/wollac/autopeering/peer"
	peerpb "github.com/wollac/autopeering/peer/proto"
	"github.com/wollac/autopeering/server"
	"go.uber.org/zap"
)

const (
	// VersionNum specifies the expected version number for this Protocol.
	VersionNum = 0

	pingExpiration     = 12 * time.Hour // time until a peer verification expires
	maxPeersInResponse = 6              // maximum number of peers returned in DiscoveryResponse
)

// The Protocol handles the peer discovery.
// It responds to incoming messages and sends own requests when needed.
type Protocol struct {
	server.Protocol

	loc *peer.Local        // local peer that runs the protocol
	log *zap.SugaredLogger // logging

	mgr       *manager // the manager handles the actual peer discovery and re-verification
	closeOnce sync.Once
}

// New creates a new discovery protocol.
func New(local *peer.Local, cfg Config) *Protocol {
	p := &Protocol{
		Protocol: server.Protocol{},
		loc:      local,
		log:      cfg.Log,
	}
	p.mgr = newManager(p, cfg.MasterPeers, cfg.Log.Named("mgr"))

	return p
}

// local returns the associated local peer of the neighbor selection.
func (p *Protocol) local() *peer.Local {
	return p.loc
}

// Start starts the actual peer discovery over the provided Sender.
func (p *Protocol) Start(s server.Sender) {
	p.Protocol.Sender = s
	p.mgr.start()

	p.log.Debugw("discover started",
		"addr", s.LocalAddr(),
	)
}

// Close finalizes the protocol.
func (p *Protocol) Close() {
	p.closeOnce.Do(func() {
		p.mgr.close()
	})
}

// IsVerified checks whether the given peer has recently been verified a recent enough endpoint proof.
func (p *Protocol) IsVerified(peer *peer.Peer) bool {
	return time.Since(p.loc.Database().LastPong(peer.ID(), peer.Address())) < pingExpiration
}

// EnsureVerified checks if the given peer has recently sent a ping;
// if not, we send a ping to trigger a verification.
func (p *Protocol) EnsureVerified(peer *peer.Peer) {
	if !p.hasVerified(peer) {
		<-p.sendPing(peer)
		// Wait for them to ping back and process our pong
		time.Sleep(server.ResponseTimeout)
	}
}

// GetVerifiedPeers returns all the currently managed peers that have been verified at least once.
func (p *Protocol) GetVerifiedPeers() []*peer.Peer {
	return unwrapPeers(p.mgr.getVerifiedPeers())
}

// HandleMessage responds to incoming peer discovery messages.
func (p *Protocol) HandleMessage(s *server.Server, from *peer.Peer, data []byte) (bool, error) {
	switch pb.MType(data[0]) {

	// Ping
	case pb.MPing:
		m := new(pb.Ping)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePing(s, from, m) {
			p.handlePing(s, from, data, m)
		}

	// Pong
	case pb.MPong:
		m := new(pb.Pong)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePong(s, from, m) {
			p.handlePong(from)
		}

	// DiscoveryRequest
	case pb.MDiscoveryRequest:
		m := new(pb.DiscoveryRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validateDiscoveryRequest(s, from, m) {
			p.handleDiscoveryRequest(s, from, data)
		}

	// DiscoveryResponse
	case pb.MDiscoveryResponse:
		m := new(pb.DiscoveryResponse)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		p.validateDiscoveryResponse(s, from, m)
		// DiscoveryResponse messages are handled in the handleReply function of the validation

	default:
		p.log.Debugw("invalid message", "type", data[0])
		return false, nil
	}

	return true, nil
}

// ------ message senders ------

// ping sends a ping to the specified peer and blocks until a reply is received or timeout.
func (p *Protocol) ping(to *peer.Peer) error {
	return <-p.sendPing(to)
}

// sendPing sends a ping to the specified address and expects a matching reply.
// This method is non-blocking, but it returns a channel that can be used to query potential errors.
func (p *Protocol) sendPing(to *peer.Peer) <-chan error {
	ping := newPing(p.LocalAddr(), to.Address())
	data := marshal(ping)

	// compute the message hash
	hash := server.PacketHash(data)
	hashEqual := func(m interface{}) bool {
		return bytes.Equal(m.(*pb.Pong).GetPingHash(), hash)
	}

	// expect a pong referencing the ping we are about to send
	return p.Protocol.SendExpectingReply(to, data, pb.MPong, hashEqual)
}

// discoveryRequest request known peers from the given target. This method blocks
// until a response is received and the provided peers are returned.
func (p *Protocol) discoveryRequest(to *peer.Peer) ([]*peer.Peer, error) {
	p.EnsureVerified(to)

	// create the request package
	req := newDiscoveryRequest(to.Address())
	data := marshal(req)

	// compute the message hash
	hash := server.PacketHash(data)
	peers := make([]*peer.Peer, 0, maxPeersInResponse)
	errc := p.Protocol.SendExpectingReply(to, data, pb.MDiscoveryResponse, func(m interface{}) bool {
		res := m.(*pb.DiscoveryResponse)
		if !bytes.Equal(res.GetReqHash(), hash) {
			return false
		}

		for _, rp := range res.GetPeers() {
			peer, err := peer.FromProto(rp)
			if err != nil {
				p.log.Warnw("invalid peer received", "err", err)
				continue
			}
			peers = append(peers, peer)
		}

		return true
	})

	// wait for the response and then return peers
	return peers, <-errc
}

// ------ helper functions ------

// hasVerified returns whether the given peer has recently verified the local peer.
func (p *Protocol) hasVerified(peer *peer.Peer) bool {
	return time.Since(p.loc.Database().LastPing(peer.ID(), peer.Address())) < pingExpiration
}

func marshal(msg pb.Message) []byte {
	mType := msg.Type()
	if mType > 0xFF {
		panic("invalid message")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		panic("invalid message")
	}
	return append([]byte{byte(mType)}, data...)
}

// ------ Packet Constructors ------

func newPing(fromAddr string, toAddr string) *pb.Ping {
	return &pb.Ping{
		Version:   VersionNum,
		From:      fromAddr,
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
}

func newPong(toAddr string, reqData []byte) *pb.Pong {
	return &pb.Pong{
		PingHash: server.PacketHash(reqData),
		To:       toAddr,
	}
}

func newDiscoveryRequest(toAddr string) *pb.DiscoveryRequest {
	return &pb.DiscoveryRequest{
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
}

func newDiscoveryResponse(reqData []byte, list []*peer.Peer) *pb.DiscoveryResponse {
	peers := make([]*peerpb.Peer, 0, len(list))
	for _, p := range list {
		peers = append(peers, p.ToProto())
	}
	return &pb.DiscoveryResponse{
		ReqHash: server.PacketHash(reqData),
		Peers:   peers,
	}
}

// ------ Packet Handlers ------

func (p *Protocol) validatePing(s *server.Server, from *peer.Peer, m *pb.Ping) bool {
	// check version number
	if m.GetVersion() != VersionNum {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"version", m.GetVersion(),
		)
		return false
	}
	// check that From matches the package sender address
	if m.GetFrom() != from.Address() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"from", m.GetFrom(),
		)
		return false
	}
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// check Timestamp
	if p.Protocol.IsExpired(m.GetTimestamp()) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"timestamp", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"from", from,
	)
	return true
}

func (p *Protocol) handlePing(s *server.Server, from *peer.Peer, rawData []byte, m *pb.Ping) {
	// create and send the pong response
	pong := newPong(from.Address(), rawData)
	s.Send(from.Address(), marshal(pong))

	// if the peer is new or expired, send a ping to verify
	if !p.IsVerified(from) {
		p.sendPing(from)
	}

	p.local().Database().UpdateLastPing(from.ID(), from.Address(), time.Now())
}

func (p *Protocol) validatePong(s *server.Server, from *peer.Peer, m *pb.Pong) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// there must be a ping waiting for this pong as a reply
	if !s.IsExpectedReply(from, m.Type(), m) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unexpected", from,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"from", from,
	)
	return true
}

func (p *Protocol) handlePong(from *peer.Peer) {
	// a valid pong verifies the peer
	p.mgr.addVerifiedPeer(from)
	// update peer database
	p.local().Database().UpdateLastPong(from.ID(), from.Address(), time.Now())
}

func (p *Protocol) validateDiscoveryRequest(s *server.Server, from *peer.Peer, m *pb.DiscoveryRequest) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// check Timestamp
	if p.Protocol.IsExpired(m.GetTimestamp()) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"timestamp", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}
	// check whether the sender is verified
	if !p.IsVerified(from) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unverified", from,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"from", from,
	)
	return true
}

func (p *Protocol) handleDiscoveryRequest(s *server.Server, from *peer.Peer, rawData []byte) {
	// get a random list of verified peers
	peers := p.mgr.getRandomPeers(maxPeersInResponse, 1)
	res := newDiscoveryResponse(rawData, peers)
	s.Send(from.Address(), marshal(res))
}

func (p *Protocol) validateDiscoveryResponse(s *server.Server, from *peer.Peer, m *pb.DiscoveryResponse) bool {
	// there must not be too many peers
	if len(m.GetPeers()) > maxPeersInResponse {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"#peers", len(m.GetPeers()),
		)
		return false
	}
	// there must be a request waiting for this response
	if !s.IsExpectedReply(from, m.Type(), m) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unexpected", from,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"from", from,
	)
	return true
}
