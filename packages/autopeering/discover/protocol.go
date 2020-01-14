package discover

import (
	"bytes"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/discover/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	peerpb "github.com/iotaledger/goshimmer/packages/autopeering/peer/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/hive.go/logger"
	"github.com/pkg/errors"
)

// The Protocol handles the peer discovery.
// It responds to incoming messages and sends own requests when needed.
type Protocol struct {
	server.Protocol

	loc *peer.Local    // local peer that runs the protocol
	log *logger.Logger // logging

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
func (p *Protocol) IsVerified(id peer.ID, addr string) bool {
	return time.Since(p.loc.Database().LastPong(id, addr)) < PingExpiration
}

// EnsureVerified checks if the given peer has recently sent a ping;
// if not, we send a ping to trigger a verification.
func (p *Protocol) EnsureVerified(peer *peer.Peer) {
	if !p.hasVerified(peer.ID(), peer.Address()) {
		<-p.sendPing(peer.Address(), peer.ID())
		// Wait for them to ping back and process our pong
		time.Sleep(server.ResponseTimeout)
	}
}

// GetVerifiedPeer returns the verified peer with the given ID, or nil if no such peer exists.
func (p *Protocol) GetVerifiedPeer(id peer.ID, addr string) *peer.Peer {
	if !p.IsVerified(id, addr) {
		return nil
	}
	peer := p.loc.Database().Peer(id)
	if peer == nil {
		return nil
	}
	if peer.Address() != addr {
		return nil
	}
	return peer
}

// GetVerifiedPeers returns all the currently managed peers that have been verified at least once.
func (p *Protocol) GetVerifiedPeers() []*peer.Peer {
	return unwrapPeers(p.mgr.getVerifiedPeers())
}

// HandleMessage responds to incoming peer discovery messages.
func (p *Protocol) HandleMessage(s *server.Server, fromAddr string, fromID peer.ID, fromKey peer.PublicKey, data []byte) (bool, error) {
	switch pb.MType(data[0]) {

	// Ping
	case pb.MPing:
		m := new(pb.Ping)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePing(s, fromAddr, m) {
			p.handlePing(s, fromAddr, fromID, fromKey, data)
		}

	// Pong
	case pb.MPong:
		m := new(pb.Pong)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePong(s, fromAddr, fromID, m) {
			p.handlePong(fromAddr, fromID, fromKey, m)
		}

	// DiscoveryRequest
	case pb.MDiscoveryRequest:
		m := new(pb.DiscoveryRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validateDiscoveryRequest(s, fromAddr, fromID, m) {
			p.handleDiscoveryRequest(s, fromAddr, data)
		}

	// DiscoveryResponse
	case pb.MDiscoveryResponse:
		m := new(pb.DiscoveryResponse)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		p.validateDiscoveryResponse(s, fromAddr, fromID, m)
		// DiscoveryResponse messages are handled in the handleReply function of the validation

	default:
		return false, nil
	}

	return true, nil
}

// ------ message senders ------

// ping sends a ping to the specified peer and blocks until a reply is received or timeout.
func (p *Protocol) ping(to *peer.Peer) error {
	return <-p.sendPing(to.Address(), to.ID())
}

// sendPing sends a ping to the specified address and expects a matching reply.
// This method is non-blocking, but it returns a channel that can be used to query potential errors.
func (p *Protocol) sendPing(toAddr string, toID peer.ID) <-chan error {
	ping := newPing(p.LocalAddr(), toAddr)
	data := marshal(ping)

	// compute the message hash
	hash := server.PacketHash(data)
	hashEqual := func(m interface{}) bool {
		return bytes.Equal(m.(*pb.Pong).GetPingHash(), hash)
	}

	// expect a pong referencing the ping we are about to send
	p.log.Debugw("send message", "type", ping.Name(), "addr", toAddr)
	errc := p.Protocol.SendExpectingReply(toAddr, toID, data, pb.MPong, hashEqual)

	return errc
}

// discoveryRequest request known peers from the given target. This method blocks
// until a response is received and the provided peers are returned.
func (p *Protocol) discoveryRequest(to *peer.Peer) ([]*peer.Peer, error) {
	p.EnsureVerified(to)

	// create the request package
	toAddr := to.Address()
	req := newDiscoveryRequest(toAddr)
	data := marshal(req)

	// compute the message hash
	hash := server.PacketHash(data)
	peers := make([]*peer.Peer, 0, MaxPeersInResponse)

	p.log.Debugw("send message", "type", req.Name(), "addr", toAddr)
	errc := p.Protocol.SendExpectingReply(toAddr, to.ID(), data, pb.MDiscoveryResponse, func(m interface{}) bool {
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
func (p *Protocol) hasVerified(id peer.ID, addr string) bool {
	return time.Since(p.loc.Database().LastPing(id, addr)) < PingExpiration
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

// createDiscoverPeer creates a new peer that only has a peering service at the given address.
func createDiscoverPeer(key peer.PublicKey, network string, address string) *peer.Peer {
	services := service.New()
	services.Update(service.PeeringKey, network, address)

	return peer.NewPeer(key, services)
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

// ------ Packet Handlers ------

func (p *Protocol) validatePing(s *server.Server, fromAddr string, m *pb.Ping) bool {
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
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", s.LocalAddr(),
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
	pong := newPong(fromAddr, rawData, s.Local().Services().CreateRecord())

	p.log.Debugw("send message",
		"type", pong.Name(),
		"addr", fromAddr,
	)
	s.Send(fromAddr, marshal(pong))

	// if the peer is new or expired, send a ping to verify
	if !p.IsVerified(fromID, fromAddr) {
		p.sendPing(fromAddr, fromID)
	} else if !p.mgr.isKnown(fromID) { // add a discovered peer to the manager if it is new
		peer := createDiscoverPeer(fromKey, p.LocalNetwork(), fromAddr)
		p.mgr.addDiscoveredPeer(peer)
	}

	_ = p.local().Database().UpdateLastPing(fromID, fromAddr, time.Now())
}

func (p *Protocol) validatePong(s *server.Server, fromAddr string, fromID peer.ID, m *pb.Pong) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", s.LocalAddr(),
		)
		return false
	}
	// there must be a ping waiting for this pong as a reply
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
	db := p.local().Database()
	_ = db.UpdateLastPong(fromID, fromAddr, time.Now())
	_ = db.UpdatePeer(from)
}

func (p *Protocol) validateDiscoveryRequest(s *server.Server, fromAddr string, fromID peer.ID, m *pb.DiscoveryRequest) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"to", m.GetTo(),
			"want", s.LocalAddr(),
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

	p.log.Debugw("send message",
		"type", res.Name(),
		"addr", fromAddr,
	)
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
