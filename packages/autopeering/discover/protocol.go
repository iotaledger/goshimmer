package discover

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/discover/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	peerpb "github.com/iotaledger/goshimmer/packages/autopeering/peer/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/hive.go/backoff"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/typeutils"
)

const (
	maxRetries = 2
	logSends   = true
)

//  policy for retrying failed network calls
var retryPolicy = backoff.ExponentialBackOff(500*time.Millisecond, 1.5).With(
	backoff.Jitter(0.5), backoff.MaxRetries(maxRetries))

// The Protocol handles the peer discovery.
// It responds to incoming messages and sends own requests when needed.
type Protocol struct {
	server.Protocol

	loc *peer.Local    // local peer that runs the protocol
	log *logger.Logger // logging

	mgr       *manager // the manager handles the actual peer discovery and re-verification
	running   *typeutils.AtomicBool
	closeOnce sync.Once
}

// New creates a new discovery protocol.
func New(local *peer.Local, cfg Config) *Protocol {
	p := &Protocol{
		Protocol: server.Protocol{},
		loc:      local,
		log:      cfg.Log,
		running:  typeutils.NewAtomicBool(),
	}
	p.mgr = newManager(p, cfg.MasterPeers, cfg.Log.Named("mgr"))

	return p
}

// Start starts the actual peer discovery over the provided Sender.
func (p *Protocol) Start(s server.Sender) {
	p.Protocol.Sender = s
	p.mgr.start()
	p.log.Debug("discover started")
	p.running.Set()
}

// Close finalizes the protocol.
func (p *Protocol) Close() {
	p.closeOnce.Do(func() {
		p.running.UnSet()
		p.mgr.close()
	})
}

// IsVerified checks whether the given peer has recently been verified a recent enough endpoint proof.
func (p *Protocol) IsVerified(id peer.ID, addr string) bool {
	return time.Since(p.loc.Database().LastPong(id, addr)) < PingExpiration
}

// EnsureVerified checks if the given peer has recently sent a Ping;
// if not, we send a Ping to trigger a verification.
func (p *Protocol) EnsureVerified(peer *peer.Peer) error {
	if !p.hasVerified(peer.ID(), peer.Address()) {
		if err := p.Ping(peer); err != nil {
			return err
		}
		// Wait for them to Ping back and process our pong
		time.Sleep(server.ResponseTimeout)
	}
	return nil
}

// GetVerifiedPeer returns the verified peer with the given ID, or nil if no such peer exists.
func (p *Protocol) GetVerifiedPeer(id peer.ID, addr string) *peer.Peer {
	for _, verified := range p.mgr.getVerifiedPeers() {
		if verified.ID() == id && verified.Address() == addr {
			return unwrapPeer(verified)
		}
	}
	return nil
}

// GetVerifiedPeers returns all the currently managed peers that have been verified at least once.
func (p *Protocol) GetVerifiedPeers() []*peer.Peer {
	return unwrapPeers(p.mgr.getVerifiedPeers())
}

// HandleMessage responds to incoming peer discovery messages.
func (p *Protocol) HandleMessage(s *server.Server, fromAddr string, fromID peer.ID, fromKey peer.PublicKey, data []byte) (bool, error) {
	if !p.running.IsSet() {
		return false, nil
	}

	switch pb.MType(data[0]) {
	// Ping
	case pb.MPing:
		m := new(pb.Ping)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		if p.validatePing(fromAddr, m) {
			p.handlePing(s, fromAddr, fromID, fromKey, data)
		}

	// Pong
	case pb.MPong:
		m := new(pb.Pong)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		if p.validatePong(s, fromAddr, fromID, m) {
			p.handlePong(fromAddr, fromID, fromKey, m)
		}

	// DiscoveryRequest
	case pb.MDiscoveryRequest:
		m := new(pb.DiscoveryRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		if p.validateDiscoveryRequest(fromAddr, fromID, m) {
			p.handleDiscoveryRequest(s, fromAddr, data)
		}

	// DiscoveryResponse
	case pb.MDiscoveryResponse:
		m := new(pb.DiscoveryResponse)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		p.validateDiscoveryResponse(s, fromAddr, fromID, m)
		// DiscoveryResponse messages are handled in the handleReply function of the validation

	default:
		return false, nil
	}

	return true, nil
}

// local returns the associated local peer of the neighbor selection.
func (p *Protocol) local() *peer.Local {
	return p.loc
}

// publicAddr returns the public address of the peering service in string representation.
func (p *Protocol) publicAddr() string {
	return p.loc.Services().Get(service.PeeringKey).String()
}

// ------ message senders ------

// Ping sends a Ping to the specified peer and blocks until a reply is received or timeout.
func (p *Protocol) Ping(to *peer.Peer) error {
	return backoff.Retry(retryPolicy, func() error {
		err := <-p.sendPing(to.Address(), to.ID())
		if err != nil && !errors.Is(err, server.ErrTimeout) {
			return backoff.Permanent(err)
		}
		return err
	})
}

// sendPing sends a Ping to the specified address and expects a matching reply.
// This method is non-blocking, but it returns a channel that can be used to query potential errors.
func (p *Protocol) sendPing(toAddr string, toID peer.ID) <-chan error {
	ping := newPing(p.publicAddr(), toAddr)
	data := marshal(ping)

	// compute the message hash
	hash := server.PacketHash(data)
	hashEqual := func(m interface{}) bool {
		return bytes.Equal(m.(*pb.Pong).GetPingHash(), hash)
	}

	p.logSend(toAddr, ping)
	return p.Protocol.SendExpectingReply(toAddr, toID, data, pb.MPong, hashEqual)
}

// DiscoveryRequest request known peers from the given target. This method blocks
// until a response is received and the provided peers are returned.
func (p *Protocol) DiscoveryRequest(to *peer.Peer) ([]*peer.Peer, error) {
	if err := p.EnsureVerified(to); err != nil {
		return nil, err
	}

	req := newDiscoveryRequest(to.Address())
	data := marshal(req)

	// compute the message hash
	hash := server.PacketHash(data)

	peers := make([]*peer.Peer, 0, MaxPeersInResponse)
	callback := func(m interface{}) bool {
		res := m.(*pb.DiscoveryResponse)
		if !bytes.Equal(res.GetReqHash(), hash) {
			return false
		}

		peers = peers[:0]
		for _, protoPeer := range res.GetPeers() {
			if p, _ := peer.FromProto(protoPeer); p != nil {
				peers = append(peers, p)
			}
		}
		return true
	}

	err := backoff.Retry(retryPolicy, func() error {
		p.logSend(to.Address(), req)
		err := <-p.Protocol.SendExpectingReply(to.Address(), to.ID(), data, pb.MDiscoveryResponse, callback)
		if err != nil && !errors.Is(err, server.ErrTimeout) {
			return backoff.Permanent(err)
		}
		return err
	})
	return peers, err
}

// ------ helper functions ------

// hasVerified returns whether the given peer has recently verified the local peer.
func (p *Protocol) hasVerified(id peer.ID, addr string) bool {
	return time.Since(p.loc.Database().LastPing(id, addr)) < PingExpiration
}

func (p *Protocol) logSend(toAddr string, msg pb.Message) {
	if logSends {
		p.log.Debugw("send message", "type", msg.Name(), "addr", toAddr)
	}
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

// newPeer creates a new peer that only has a peering service at the given address.
func newPeer(key peer.PublicKey, network string, address string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, network, address)

	return peer.NewPeer(key, services)
}

// ------ Message Constructors ------

func newPing(fromAddr string, toAddr string) *pb.Ping {
	return &pb.Ping{
		Version:   VersionNum,
		From:      fromAddr,
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
}

func newPong(toAddr string, reqData []byte, services *service.Record) *pb.Pong {
	return &pb.Pong{
		PingHash: server.PacketHash(reqData),
		To:       toAddr,
		Services: services.ToProto(),
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

// ------ Message Handlers ------

func (p *Protocol) validatePing(fromAddr string, m *pb.Ping) bool {
	// check version number
	if m.GetVersion() != VersionNum {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"version", m.GetVersion(),
			"want", VersionNum,
		)
		return false
	}
	// check that From matches the package sender address
	if m.GetFrom() != fromAddr {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"from", m.GetFrom(),
			"want", fromAddr,
		)
		return false
	}
	// check that To matches the local address
	if m.GetTo() != p.publicAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", p.publicAddr(),
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
		"addr", fromAddr,
	)
	return true
}

func (p *Protocol) handlePing(s *server.Server, fromAddr string, fromID peer.ID, fromKey peer.PublicKey, rawData []byte) {
	// create and send the pong response
	pong := newPong(fromAddr, rawData, p.loc.Services().CreateRecord())

	p.logSend(fromAddr, pong)
	s.Send(fromAddr, marshal(pong))

	// if the peer is new or expired, send a Ping to verify
	if !p.IsVerified(fromID, fromAddr) {
		p.sendPing(fromAddr, fromID)
	} else if !p.mgr.isKnown(fromID) { // add a discovered peer to the manager if it is new
		p.mgr.addDiscoveredPeer(newPeer(fromKey, s.LocalAddr().Network(), fromAddr))
	}

	_ = p.loc.Database().UpdateLastPing(fromID, fromAddr, time.Now())
}

func (p *Protocol) validatePong(s *server.Server, fromAddr string, fromID peer.ID, m *pb.Pong) bool {
	// check that To matches the local address
	if m.GetTo() != p.publicAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", p.publicAddr(),
		)
		return false
	}
	// there must be a Ping waiting for this pong as a reply
	if !s.IsExpectedReply(fromAddr, fromID, m.Type(), m) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unexpected", fromAddr,
		)
		return false
	}
	// there must a valid number of services
	numServices := len(m.GetServices().GetMap())
	if numServices <= 0 || numServices > MaxServices {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"#peers", numServices,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"addr", fromAddr,
	)
	return true
}

func (p *Protocol) handlePong(fromAddr string, fromID peer.ID, fromKey peer.PublicKey, m *pb.Pong) {
	services, _ := service.FromProto(m.GetServices())
	peering := services.Get(service.PeeringKey)
	if peering == nil || peering.String() != fromAddr {
		p.log.Warn("invalid services")
		return
	}

	// create a proper key with these services
	from := peer.NewPeer(fromKey, services)

	// a valid pong verifies the peer
	p.mgr.addVerifiedPeer(from)

	// update peer database
	db := p.loc.Database()
	_ = db.UpdateLastPong(fromID, fromAddr, time.Now())
	_ = db.UpdatePeer(from)
}

func (p *Protocol) validateDiscoveryRequest(fromAddr string, fromID peer.ID, m *pb.DiscoveryRequest) bool {
	// check that To matches the local address
	if m.GetTo() != p.publicAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", p.publicAddr(),
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
	if !p.IsVerified(fromID, fromAddr) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unverified", fromAddr,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"addr", fromAddr,
	)
	return true
}

func (p *Protocol) handleDiscoveryRequest(s *server.Server, fromAddr string, rawData []byte) {
	// get a random list of verified peers
	peers := p.mgr.getRandomPeers(MaxPeersInResponse, 1)
	res := newDiscoveryResponse(rawData, peers)

	p.logSend(fromAddr, res)
	s.Send(fromAddr, marshal(res))
}

func (p *Protocol) validateDiscoveryResponse(s *server.Server, fromAddr string, fromID peer.ID, m *pb.DiscoveryResponse) bool {
	// there must not be too many peers
	if len(m.GetPeers()) > MaxPeersInResponse {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"#peers", len(m.GetPeers()),
		)
		return false
	}
	// there must be a request waiting for this response
	if !s.IsExpectedReply(fromAddr, fromID, m.Type(), m) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unexpected", fromAddr,
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"addr", fromAddr,
	)
	return true
}
