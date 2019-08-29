package discover

import (
	"fmt"
	"time"

	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/peer"
	peerpb "github.com/wollac/autopeering/peer/proto"
	pb "github.com/wollac/autopeering/proto"
)

// isVerified checks if the given node has a recent enough endpoint proof.
func (s *Server) isVerified(p *peer.Peer) bool {
	return time.Since(s.mgr.db.LastPong(p)) < pongExpiration
}

// ensureVerified checks if the given node has recently sent a ping;
// if not, we send a ping to trigger a verification.
func (s *Server) ensureVerified(p *peer.Peer) {
	if time.Since(s.mgr.db.LastPing(p)) >= pongExpiration {
		<-s.sendPing(p, nil)
		// Wait for them to ping back and process our pong
		time.Sleep(responseTimeout)
	}
}

// expired checks whether the given UNIX time stamp is too far in the past.
func expired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= packetExpiration
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
		PingHash: packetHash(reqData),
		To:       toAddr,
	}
}

func newPeersRequest() *pb.PeersRequest {
	return &pb.PeersRequest{
		Timestamp: time.Now().Unix(),
	}
}

func newPeersResponse(reqData []byte, resPeers []*peer.Peer) *pb.PeersResponse {
	peers := make([]*peerpb.Peer, 0, len(resPeers))
	for _, s := range resPeers {
		peers = append(peers, &peerpb.Peer{
			PublicKey: s.Identity.PublicKey,
			Address:   s.Address,
		})
	}

	return &pb.PeersResponse{
		ReqHash: packetHash(reqData),
		Peers:   peers,
	}
}

// ------ Packet Handlers ------

func (s *Server) handlePacket(fromAddr string, pkt *pb.Packet) error {
	w, fromID, err := decode(pkt)
	if err != nil {
		return err
	}
	if w.GetMessage() == nil {
		return errNoMessage
	}

	switch m := w.GetMessage().(type) {

	// Ping
	case *pb.MessageWrapper_Ping:
		s.log.Debugw("handle "+m.Ping.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePing(m.Ping, fromAddr) {
			s.handlePing(m.Ping, fromID, fromAddr, pkt.GetData())
		}

	// Pong
	case *pb.MessageWrapper_Pong:
		s.log.Debugw("handle "+m.Pong.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePong(m.Pong, fromID, fromAddr) {
			s.handlePong(m.Pong, fromID, fromAddr)
		}

	// PeersRequest
	case *pb.MessageWrapper_PeersRequest:
		s.log.Debugw("handle "+m.PeersRequest.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePeersRequest(m.PeersRequest, fromID, fromAddr) {
			s.handlePeersRequest(m.PeersRequest, fromID, fromAddr, pkt.GetData())
		}

	// PeersResponse
	case *pb.MessageWrapper_PeersResponse:
		s.log.Debugw("handle "+m.PeersResponse.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePeersResponse(m.PeersResponse, fromID, fromAddr) {
			s.handlePeersResponse(m.PeersResponse, fromID, fromAddr)
		}

	default:
		panic(fmt.Sprintf("invalid message type: %T", w.GetMessage()))
	}

	return nil
}

func (s *Server) validatePing(m *pb.Ping, fromAddr string) bool {
	// check version number
	if m.GetVersion() != VersionNum {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"version", m.GetVersion(),
		)
		return false
	}
	// check that From matches the package sender address
	if m.GetFrom() != fromAddr {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"from", m.GetFrom(),
		)
		return false
	}
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// check Timestamp
	if expired(m.GetTimestamp()) {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"ts", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}
	return true
}

func (s *Server) handlePing(m *pb.Ping, fromID *id.Identity, fromAddr string, rawData []byte) {
	// update the peer
	p := peer.NewPeer(fromID, fromAddr)
	s.mgr.db.UpdateLastPing(p, time.Now())

	// create and send the pong response
	pong := newPong(fromAddr, rawData)
	s.send(fromAddr, pong)

	// if the peer is new or expired, send a ping to verify
	if !s.isVerified(p) {
		s.sendPing(p, nil)
	}
}

func (s *Server) validatePong(m *pb.Pong, fromID *id.Identity, fromAddr string) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// there must be a ping waiting for this pong as a reply
	if !s.handleReply(fromAddr, fromID, m) {
		s.log.Debugw("no matching request",
			"type", m.Name(),
			"from", fromAddr,
		)
		return false
	}
	return true
}

func (s *Server) handlePong(m *pb.Pong, fromID *id.Identity, fromAddr string) {
	p := peer.NewPeer(fromID, fromAddr)
	s.mgr.db.UpdateLastPong(p, time.Now())
	// a valid pong verifies the peer
	s.mgr.addVerifiedPeer(p)
}

func (s *Server) validatePeersRequest(m *pb.PeersRequest, fromID *id.Identity, fromAddr string) bool {
	// check Timestamp
	if expired(m.GetTimestamp()) {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"ts", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}
	if !s.isVerified(peer.NewPeer(fromID, fromAddr)) {
		s.log.Debugw("failed to validate",
			"type", m.Name(),
			"id", fromID,
			"addr", fromAddr,
		)
		return false
	}
	return true
}

func (s *Server) handlePeersRequest(m *pb.PeersRequest, fromID *id.Identity, fromAddr string, rawData []byte) {
	peers := s.mgr.getRandomPeers(maxPeersInResponse)
	s.send(fromAddr, newPeersResponse(rawData, peers))
}

func (s *Server) validatePeersResponse(m *pb.PeersResponse, fromID *id.Identity, fromAddr string) bool {
	// there must not be too many peers
	if len(m.GetPeers()) > maxPeersInResponse {
		return false
	}
	// there must be a request waiting for this response
	if !s.handleReply(fromAddr, fromID, m) {
		s.log.Debugw("no matching request",
			"type", m.Name(),
			"from", fromAddr,
		)
		return false
	}
	return true
}

func (s *Server) handlePeersResponse(m *pb.PeersResponse, fromID *id.Identity, fromAddr string) {
	for _, p := range m.GetPeers() {
		identity, err := id.NewIdentity(p.PublicKey)
		if err != nil {
			s.log.Warnw("invalid public key", "err", err)
			continue
		}
		s.mgr.addDiscoveredPeer(peer.NewPeer(identity, p.Address))
	}
}
