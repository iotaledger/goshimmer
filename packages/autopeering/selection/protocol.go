package selection

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/selection/proto"
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

// DiscoverProtocol specifies the methods from the peer discovery that are required.
type DiscoverProtocol interface {
	IsVerified(id peer.ID, addr string) bool
	EnsureVerified(*peer.Peer) error

	GetVerifiedPeer(id peer.ID, addr string) *peer.Peer
	GetVerifiedPeers() []*peer.Peer
}

// The Protocol handles the neighbor selection.
// It responds to incoming messages and sends own requests when needed.
type Protocol struct {
	server.Protocol

	disc DiscoverProtocol // reference to the peer discovery to query verified peers
	loc  *peer.Local      // local peer that runs the protocol
	log  *logger.Logger   // logging

	mgr       *manager // the manager handles the actual neighbor selection
	running   *typeutils.AtomicBool
	closeOnce sync.Once
}

// New creates a new neighbor selection protocol.
func New(local *peer.Local, disc DiscoverProtocol, cfg Config) *Protocol {
	p := &Protocol{
		Protocol: server.Protocol{},
		loc:      local,
		disc:     disc,
		log:      cfg.Log,
		running:  typeutils.NewAtomicBool(),
	}
	p.mgr = newManager(p, disc.GetVerifiedPeers, cfg.Log.Named("mgr"), cfg)

	return p
}

// Start starts the actual neighbor selection over the provided Sender.
func (p *Protocol) Start(s server.Sender) {
	p.Protocol.Sender = s
	p.mgr.start()
	p.log.Debug("neighborhood started")
	p.running.Set()
}

// Close finalizes the protocol.
func (p *Protocol) Close() {
	p.closeOnce.Do(func() {
		p.running.UnSet()
		p.mgr.close()
	})
}

// GetNeighbors returns the current neighbors.
func (p *Protocol) GetNeighbors() []*peer.Peer {
	return p.mgr.getNeighbors()
}

// GetIncomingNeighbors returns the current incoming neighbors.
func (p *Protocol) GetIncomingNeighbors() []*peer.Peer {
	return p.mgr.getInNeighbors()
}

// GetOutgoingNeighbors returns the current outgoing neighbors.
func (p *Protocol) GetOutgoingNeighbors() []*peer.Peer {
	return p.mgr.getOutNeighbors()
}

// RemoveNeighbor removes the peer with the given id from the incoming and outgoing neighbors.
// If such a peer was actually contained in anyone of the neighbor sets, the corresponding event is triggered
// and the corresponding peering drop is sent. Otherwise the call is ignored.
func (p *Protocol) RemoveNeighbor(id peer.ID) {
	p.mgr.removeNeighbor(id)
}

// HandleMessage responds to incoming neighbor selection messages.
func (p *Protocol) HandleMessage(s *server.Server, fromAddr string, fromID peer.ID, _ peer.PublicKey, data []byte) (bool, error) {
	if !p.running.IsSet() {
		return false, nil
	}

	switch pb.MType(data[0]) {
	// PeeringRequest
	case pb.MPeeringRequest:
		m := new(pb.PeeringRequest)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		if p.validatePeeringRequest(fromAddr, fromID, m) {
			p.handlePeeringRequest(s, fromAddr, fromID, data, m)
		}

	// PeeringResponse
	case pb.MPeeringResponse:
		m := new(pb.PeeringResponse)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		p.validatePeeringResponse(s, fromAddr, fromID, m)
		// PeeringResponse messages are handled in the handleReply function of the validation

	// PeeringDrop
	case pb.MPeeringDrop:
		m := new(pb.PeeringDrop)
		if err := proto.Unmarshal(data[1:], m); err != nil {
			return true, fmt.Errorf("invalid message: %w", err)
		}
		if p.validatePeeringDrop(fromAddr, m) {
			p.handlePeeringDrop(fromID)
		}

	default:
		return false, nil
	}

	return true, nil
}

// Local returns the associated local peer of the neighbor selection.
func (p *Protocol) local() *peer.Local {
	return p.loc
}

// publicAddr returns the public address of the peering service in string representation.
func (p *Protocol) publicAddr() string {
	return p.loc.Services().Get(service.PeeringKey).String()
}

// ------ message senders ------

// PeeringRequest sends a PeeringRequest to the given peer. This method blocks
// until a response is received and the status answer is returned.
func (p *Protocol) PeeringRequest(to *peer.Peer, salt *salt.Salt) (bool, error) {
	if err := p.disc.EnsureVerified(to); err != nil {
		return false, err
	}

	// create the request package
	toAddr := to.Address()
	req := newPeeringRequest(toAddr, salt)
	data := marshal(req)

	// compute the message hash
	hash := server.PacketHash(data)

	var status bool
	callback := func(m interface{}) bool {
		res := m.(*pb.PeeringResponse)
		if !bytes.Equal(res.GetReqHash(), hash) {
			return false
		}
		status = res.GetStatus()
		return true
	}

	err := backoff.Retry(retryPolicy, func() error {
		p.logSend(toAddr, req)
		err := <-p.Protocol.SendExpectingReply(toAddr, to.ID(), data, pb.MPeeringResponse, callback)
		if err != nil && !errors.Is(err, server.ErrTimeout) {
			return backoff.Permanent(err)
		}
		return err
	})
	return status, err
}

// PeeringDrop sends a peering drop message to the given peer, non-blocking and does not wait for any responses.
func (p *Protocol) PeeringDrop(to *peer.Peer) {
	drop := newPeeringDrop(to.Address())

	p.logSend(to.Address(), drop)
	p.Protocol.Send(to, marshal(drop))
}

// ------ helper functions ------

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

// ------ Message Constructors ------

func newPeeringRequest(toAddr string, salt *salt.Salt) *pb.PeeringRequest {
	return &pb.PeeringRequest{
		To:        toAddr,
		Timestamp: time.Now().Unix(),
		Salt:      salt.ToProto(),
	}
}

func newPeeringResponse(reqData []byte, status bool) *pb.PeeringResponse {
	return &pb.PeeringResponse{
		ReqHash: server.PacketHash(reqData),
		Status:  status,
	}
}

func newPeeringDrop(toAddr string) *pb.PeeringDrop {
	return &pb.PeeringDrop{
		To:        toAddr,
		Timestamp: time.Now().Unix(),
	}
}

// ------ Packet Handlers ------

func (p *Protocol) validatePeeringRequest(fromAddr string, fromID peer.ID, m *pb.PeeringRequest) bool {
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
	if !p.disc.IsVerified(fromID, fromAddr) {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"unverified", fromAddr,
		)
		return false
	}
	// check Salt
	s, err := salt.FromProto(m.GetSalt())
	if err != nil {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"salt", err,
		)
		return false
	}
	// check salt expiration
	if s.Expired() {
		p.log.Debugw("invalid message",
			"type", m.Name(),
			"salt.expiration", s.GetExpiration(),
		)
		return false
	}

	p.log.Debugw("valid message",
		"type", m.Name(),
		"addr", fromAddr,
	)
	return true
}

func (p *Protocol) handlePeeringRequest(s *server.Server, fromAddr string, fromID peer.ID, rawData []byte, m *pb.PeeringRequest) {
	salt, err := salt.FromProto(m.GetSalt())
	if err != nil {
		// this should not happen as it is checked in validation
		p.log.Warnw("invalid salt", "err", err)
		return
	}

	from := p.disc.GetVerifiedPeer(fromID, fromAddr)
	if from == nil {
		// this should not happen as it is checked in validation
		p.log.Warnw("invalid stored peer",
			"id", fromID,
		)
		return
	}

	res := newPeeringResponse(rawData, p.mgr.requestPeering(from, salt))

	p.logSend(fromAddr, res)
	s.Send(fromAddr, marshal(res))
}

func (p *Protocol) validatePeeringResponse(s *server.Server, fromAddr string, fromID peer.ID, m *pb.PeeringResponse) bool {
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

func (p *Protocol) validatePeeringDrop(fromAddr string, m *pb.PeeringDrop) bool {
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

func (p *Protocol) handlePeeringDrop(fromID peer.ID) {
	p.mgr.removeNeighbor(fromID)
}
