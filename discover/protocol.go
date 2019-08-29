package discover

import (
	"time"

	"github.com/wollac/autopeering/id"
	peerpb "github.com/wollac/autopeering/peer/proto"
	pb "github.com/wollac/autopeering/proto"
)

func (s *Server) handlePacket(fromAddr string, pkt *pb.Packet) error {
	w, fromID, err := decode(pkt)
	if err != nil {
		return err
	}

	switch m := w.GetMessage().(type) {

	// Ping
	case *pb.MessageWrapper_Ping:
		s.log.Debugw("handle "+m.Ping.Name(), "id", fromID, "addr", fromAddr)
		if s.verifyPing(m.Ping, fromAddr) {
			s.handlePing(m.Ping, fromID, fromAddr, pkt.GetData())
		}

	// Pong
	case *pb.MessageWrapper_Pong:
		s.log.Debugw("handle "+m.Pong.Name(), "id", fromID, "addr", fromAddr)
		if s.verifyPong(m.Pong, fromID, fromAddr) {
			s.handlePong(m.Pong, fromID, fromAddr)
		}

	// PeersRequest
	case *pb.MessageWrapper_PeersRequest:
		s.log.Debugw("handle "+m.PeersRequest.Name(), "id", fromID, "addr", fromAddr)
		if s.verifyPeersRequest(m.PeersRequest, fromID, fromAddr) {
			s.handlePeersRequest(m.PeersRequest, fromID, fromAddr, pkt.GetData())
		}

	// PeersResponse
	case *pb.MessageWrapper_PeersResponse:
		s.log.Debugw("handle "+m.PeersResponse.Name(), "id", fromID, "addr", fromAddr)
		if s.verifyPeersResponse(m.PeersResponse, fromID, fromAddr) {
			s.handlePeersResponse(m.PeersResponse, fromID, fromAddr)
		}

	default:
		panic("invalid pb type")
	}

	return nil
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

func newPeersResponse(reqData []byte, resPeers []*Peer) *pb.PeersResponse {
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

func (s *Server) verifyPing(m *pb.Ping, fromAddr string) bool {
	// check version number
	if m.GetVersion() != VersionNum {
		s.log.Debugw("failed to verify",
			"type", m.Name(),
			"version", m.GetVersion(),
		)
		return false
	}
	// check that From matches the package sender address
	if m.GetFrom() != fromAddr {
		s.log.Debugw("failed to verify",
			"type", m.Name(),
			"from", m.GetFrom(),
		)
		return false
	}
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		s.log.Debugw("failed to verify",
			"type", m.Name(),
			"to", m.GetTo(),
		)
		return false
	}
	// check Timestamp
	if expired(m.GetTimestamp()) {
		s.log.Debugw("failed to verify",
			"type", m.Name(),
			"ts", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}
	return true
}

func (s *Server) handlePing(m *pb.Ping, fromID *id.Identity, fromAddr string, rawData []byte) {
	// create the pong package
	pong := newPong(fromAddr, rawData)
	s.send(fromAddr, pong)

	peer := NewPeer(fromID, fromAddr)
	s.mgr.db.UpdateLastPing(peer, time.Now())

	if time.Since(s.mgr.db.LastPong(peer)) > pongExpiration {
		s.sendPing(peer, func() {
			s.mgr.addVerifiedPeer(peer)
		})
	} else {
		s.mgr.addVerifiedPeer(peer)
	}

}

func (s *Server) verifyPong(m *pb.Pong, fromID *id.Identity, fromAddr string) bool {
	// check that To matches the local address
	if m.GetTo() != s.LocalAddr() {
		s.log.Debugw("failed to verify",
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
	s.mgr.db.UpdateLastPong(NewPeer(fromID, fromAddr), time.Now())
}

func (s *Server) verifyPeersRequest(m *pb.PeersRequest, fromID *id.Identity, fromAddr string) bool {
	// check Timestamp
	if expired(m.GetTimestamp()) {
		s.log.Debugw("failed to verify",
			"type", m.Name(),
			"ts", time.Unix(m.GetTimestamp(), 0),
		)
		return false
	}
	if !s.checkBond(NewPeer(fromID, fromAddr)) {
		s.log.Debugw("failed to verify",
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

func (s *Server) verifyPeersResponse(m *pb.PeersResponse, fromID *id.Identity, fromAddr string) bool {
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
	for _, peer := range m.GetPeers() {
		identity, err := id.NewIdentity(peer.PublicKey)
		if err != nil {
			s.log.Warnw("invalid public key", "err", err)
			continue
		}
		s.mgr.addDiscoveredPeer(NewPeer(identity, peer.Address))
	}
}
