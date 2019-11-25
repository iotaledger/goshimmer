package selection

import (
	"bytes"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/autopeering-sim/salt"
	pb "github.com/iotaledger/autopeering-sim/selection/proto"
	"github.com/iotaledger/autopeering-sim/server"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// DiscoverProtocol specifies the methods from the peer discovery that are required.
type DiscoverProtocol interface {
	IsVerified(*peer.Peer) bool
	EnsureVerified(*peer.Peer)

	GetVerifiedPeers() []*peer.Peer
}

// The Protocol handles the neighbor selection.
// It responds to incoming messages and sends own requests when needed.
type Protocol struct {
	server.Protocol

	disc DiscoverProtocol   // reference to the peer discovery to query verified peers
	loc  *peer.Local        // local peer that runs the protocol
	log  *zap.SugaredLogger // logging

	mgr       *manager // the manager handles the actual neighbor selection
	closeOnce sync.Once
}

// New creates a new neighbor selection protocol.
func New(local *peer.Local, disc DiscoverProtocol, cfg Config) *Protocol {
	p := &Protocol{
		Protocol: server.Protocol{},
		loc:      local,
		disc:     disc,
		log:      cfg.Log,
	}
	p.mgr = newManager(p, disc.GetVerifiedPeers, cfg.Log.Named("mgr"), cfg.Param)

	return p
}

// Local returns the associated local peer of the neighbor selection.
func (p *Protocol) local() *peer.Local {
	return p.loc
}

// Start starts the actual neighbor selection over the provided Sender.
func (p *Protocol) Start(s server.Sender) {
	p.Protocol.Sender = s
	p.mgr.start()

	p.log.Debugw("neighborhood started",
		"addr", s.LocalAddr(),
	)
}

// Close finalizes the protocol.
func (p *Protocol) Close() {
	p.closeOnce.Do(func() {
		p.mgr.close()
	})
}

// GetNeighbors returns the current neighbors.
func (p *Protocol) GetNeighbors() []*peer.Peer {
	return p.mgr.getNeighbors()
}

// GetNeighbors returns the current incoming neighbors.
func (p *Protocol) GetIncomingNeighbors() []*peer.Peer {
	return p.mgr.getIncomingNeighbors()
}

// GetNeighbors returns the current outgoing neighbors.
func (p *Protocol) GetOutgoingNeighbors() []*peer.Peer {
	return p.mgr.getOutgoingNeighbors()
}

// HandleMessage responds to incoming neighbor selection messages.
func (p *Protocol) HandleMessage(s *server.Server, from *peer.Peer, data []byte) (bool, error) {
	switch pb.MType(data[0]) {

	// PeeringRequest
	case pb.MPeeringRequest:
		m := new(pb.PeeringRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePeeringRequest(s, from, m) {
			p.handlePeeringRequest(s, from, data, m)
		}

	// PeeringResponse
	case pb.MPeeringResponse:
		m := new(pb.PeeringResponse)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		p.validatePeeringResponse(s, from, m)
		// PeeringResponse messages are handled in the handleReply function of the validation

	// PeeringDrop
	case pb.MPeeringDrop:
		m := new(pb.PeeringDrop)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, errors.Wrap(err, "invalid message")
		}
		if p.validatePeeringDrop(s, from, m) {
			p.handlePeeringDrop(from)
		}

	default:
		p.log.Debugw("invalid message", "type", data[0])
		return false, nil
	}

	return true, nil
}

// ------ message senders ------

// RequestPeering sends a peering request to the given peer. This method blocks
// until a response is received and the status answer is returned.
func (p *Protocol) RequestPeering(to *peer.Peer, salt *salt.Salt, myServices peer.ServiceMap) (peer.ServiceMap, error) {
	p.disc.EnsureVerified(to)

	// create the request package
	req := newPeeringRequest(to.Address(), salt, myServices)
	data := marshal(req)

	// compute the message hash
	hash := server.PacketHash(data)

	var services peer.ServiceMap
	callback := func(m interface{}) bool {
		res := m.(*pb.PeeringResponse)
		if !bytes.Equal(res.GetReqHash(), hash) {
			return false
		}
		if res.GetStatus() {
			services = peer.NewServiceMapFromProto(res.GetServices())
		}
		return true
	}

	err := <-p.Protocol.SendExpectingReply(to, data, pb.MPeeringResponse, callback)
	return services, err
}

// DropPeer sends a PeeringDrop to the given peer.
func (p *Protocol) DropPeer(to *peer.Peer) {
	toAddr := to.Address()

	drop := newPeeringDrop(toAddr)
	p.Protocol.Send(to, marshal(drop))
}

// ------ helper functions ------

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

func newPeeringRequest(toAddr string, salt *salt.Salt, services peer.ServiceMap) *pb.PeeringRequest {
	return &pb.PeeringRequest{
		To:        toAddr,
		Timestamp: time.Now().Unix(),
		Salt:      salt.ToProto(),
		Services:  peer.ServiceMapToProto(services),
	}
}

func newPeeringResponse(reqData []byte, services peer.ServiceMap) *pb.PeeringResponse {
	return &pb.PeeringResponse{
		ReqHash:  server.PacketHash(reqData),
		Status:   len(services) > 0,
		Services: peer.ServiceMapToProto(services),
	}
}

func newPeeringDrop(toAddr string) *pb.PeeringDrop {
	return &pb.PeeringDrop{
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
}

// ------ Packet Handlers ------

func (p *Protocol) validatePeeringRequest(s *server.Server, from *peer.Peer, m *pb.PeeringRequest) bool {
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
	if !p.disc.IsVerified(from) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unverified", from,
		)
		return false
	}
	// check Salt
	salt, err := salt.FromProto(m.GetSalt())
	if err != nil {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"salt", err,
		)
		return false
	}
	// check salt expiration
	if salt.Expired() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"salt.expiration", salt.GetExpiration(),
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"from", from,
		"services", peer.NewServiceMapFromProto(m.GetServices()),
	)
	return true
}

func (p *Protocol) handlePeeringRequest(s *server.Server, from *peer.Peer, rawData []byte, m *pb.PeeringRequest) {
	salt, err := salt.FromProto(m.GetSalt())
	if err != nil {
		// this should not happen as it is checked in validation
		p.log.Warnw("invalid salt", "err", err)
		return
	}

	services := peer.NewServiceMapFromProto(m.GetServices())

	res := newPeeringResponse(rawData, p.mgr.acceptRequest(from, salt, services))
	s.Send(from.Address(), marshal(res))
}

func (p *Protocol) validatePeeringResponse(s *server.Server, from *peer.Peer, m *pb.PeeringResponse) bool {
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
		"services", peer.NewServiceMapFromProto(m.GetServices()),
	)
	return true
}

func (p *Protocol) validatePeeringDrop(s *server.Server, from *peer.Peer, m *pb.PeeringDrop) bool {
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

func (p *Protocol) handlePeeringDrop(from *peer.Peer) {
	p.mgr.dropNeighbor(from.ID())
}
