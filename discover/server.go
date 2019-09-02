package discover

import (
	"container/list"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/peer"
	pb "github.com/wollac/autopeering/proto"
	"github.com/wollac/autopeering/salt"
	"github.com/wollac/autopeering/transport"
	"go.uber.org/zap"
)

const (
	// VersionNum specifies the expected version number for this protocol.
	VersionNum = 0

	responseTimeout  = 500 * time.Millisecond
	packetExpiration = 20 * time.Second
	pongExpiration   = 12 * time.Hour

	maxPeersInResponse = 6 // maximum number of peers returned in PeersResponse
)

// Server offers the functionality to start a discovery server.
type Server struct {
	trans   transport.Transport
	local   *peer.Local
	mgr     *manager
	log     *zap.SugaredLogger
	address string

	acceptRequest func(*peer.Peer, *salt.Salt) bool
	dropReceived  chan<- peer.ID

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{} // if this channel gets closed all pending waits should terminate
}

// replyMatchFunc is the type of the matcher callback. If it returns matched, the
// reply was acceptable. If requestDone is false, the reply is considered
// incomplete and the callback will be invoked again for the next matching reply.
type replyMatchFunc func(pb.Message) (matched bool, requestDone bool)

type replyMatcher struct {
	// fromAddr must match the sender of the reply
	fromAddr string
	// fromID must match the sender ID
	fromID peer.ID
	// mtype must match the type of the reply
	mtype pb.MType

	// deadline when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives
	callback replyMatchFunc

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout
	errc chan error
}

// reply is a reply packet from a certain peer
type reply struct {
	fromAddr string
	fromID   peer.ID
	message  pb.Message

	// a matching request is indicated via this channel
	matched chan<- bool
}

// Listen starts a new peer discovery server using the given transport layer for communication.
func Listen(t transport.Transport, local *peer.Local, cfg Config) *Server {
	srv := &Server{
		trans:           t,
		local:           local,
		log:             cfg.Log,
		acceptRequest:   cfg.AcceptRequest,
		dropReceived:    cfg.DropReceived,
		address:         t.LocalAddr(),
		addReplyMatcher: make(chan *replyMatcher),
		gotreply:        make(chan reply),
		closing:         make(chan struct{}),
	}
	srv.mgr = newManager(srv, local.Database(), cfg.Bootnodes, cfg.Log.Named("mgr"))

	srv.wg.Add(2)
	go srv.replyLoop()
	go srv.readLoop()

	return srv
}

// Close shuts down the server.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.closing)
		s.mgr.close()
		s.trans.Close()
		s.wg.Wait()
	})
}

// Local returns the the local peer.
func (s *Server) Local() *peer.Local {
	return s.local
}

// LocalAddr returns the address of the local peer in string form.
func (s *Server) LocalAddr() string {
	return s.address
}

func (s *Server) self() peer.ID {
	return s.Local().ID()
}

// Loop checking for matching replies.
func (s *Server) replyLoop() {
	defer s.wg.Done()

	var (
		mlist   = list.New()
		timeout = time.NewTimer(0)
	)
	defer timeout.Stop()

	<-timeout.C // ignore first timeout

	for {

		// Set the timer so that it fires when the next reply expires
		if el := mlist.Front(); el != nil {
			// the first element always has the closest deadline
			m := el.Value.(*replyMatcher)
			timeout.Reset(time.Until(m.deadline))
		} else {
			timeout.Stop()
		}

		select {

		// add a new matcher to the list
		case s := <-s.addReplyMatcher:
			s.deadline = time.Now().Add(responseTimeout)
			mlist.PushBack(s)

		// on reply received, check all matchers for fits
		case r := <-s.gotreply:
			rtype := r.message.Type()
			matched := false
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*replyMatcher)
				if m.mtype == rtype && m.fromID == r.fromID && m.fromAddr == r.fromAddr {
					ok, requestDone := m.callback(r.message)
					matched = matched || ok

					if requestDone {
						m.errc <- nil
						mlist.Remove(el)
					}
				}
			}
			r.matched <- matched

		// on timeout, check for expired matchers
		case <-timeout.C:
			now := time.Now()

			// notify and remove any expired matchers
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*replyMatcher)
				if now.After(m.deadline) || now.Equal(m.deadline) {
					m.errc <- errTimeout
					mlist.Remove(el)
				}
			}

		// on close, notice all the matchers
		case <-s.closing:
			for el := mlist.Front(); el != nil; el = el.Next() {
				el.Value.(*replyMatcher).errc <- errClosed
			}
			return

		}
	}
}

// Expects a reply message with the given specifications.
// If eventually nil is returned, a matching message was received.
func (s *Server) expectReply(fromAddr string, fromID peer.ID, mtype pb.MType, callback replyMatchFunc) <-chan error {
	ch := make(chan error, 1)
	m := &replyMatcher{fromAddr: fromAddr, fromID: fromID, mtype: mtype, callback: callback, errc: ch}
	select {
	case s.addReplyMatcher <- m:
	case <-s.closing:
		ch <- errClosed
	}
	return ch
}

// Process a reply message and returns whether a matching request could be found.
func (s *Server) handleReply(fromAddr string, fromID peer.ID, message pb.Message) bool {
	matched := make(chan bool, 1)
	select {
	case s.gotreply <- reply{fromAddr, fromID, message, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-s.closing:
		return false
	}
}

// send a messge to the given address
func (s *Server) send(toAddr string, msg pb.Message) {
	pkt := s.encode(msg)
	s.write(toAddr, msg.Name(), pkt)
}

func (s *Server) write(toAddr string, mName string, pkt *pb.Packet) {
	err := s.trans.WriteTo(pkt, toAddr)
	s.log.Debugw("write "+mName, "to", toAddr, "err", err)
}

// encodes a message as a data packet that can be written.
func (s *Server) encode(message pb.Message) *pb.Packet {
	// wrap the message before marshaling
	data, err := proto.Marshal(message.Wrapper())
	if err != nil {
		panic(fmt.Sprintf("protobuf error: %v", err))
	}

	return &pb.Packet{
		PublicKey: s.local.PublicKey(),
		Signature: s.local.Sign(data),
		Data:      data,
	}
}

func (s *Server) readLoop() {
	defer s.wg.Done()

	for {
		pkt, fromAddr, err := s.trans.ReadFrom()
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			s.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			if err != io.EOF {
				s.log.Warnw("read error", "err", err)
			}
			s.log.Debug("reading stopped")
			return
		}

		if err := s.handlePacket(pkt, fromAddr); err != nil {
			s.log.Warnw("failed to handle packet", "from", fromAddr, "err", err)
		}
	}
}

func (s *Server) handlePacket(pkt *pb.Packet, fromAddr string) error {
	w, fromKey, err := decode(pkt)
	if err != nil {
		return err
	}
	if w.GetMessage() == nil {
		return errNoMessage
	}
	fromID := fromKey.ID()

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
			s.handlePong(m.Pong, fromID, fromAddr, fromKey)
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
		s.validatePeersResponse(m.PeersResponse, fromID, fromAddr)
		// PeersResponse messages are handled in the handleReply function of the validation

	// PeeringRequest
	case *pb.MessageWrapper_PeeringRequest:
		s.log.Debugw("handle "+m.PeeringRequest.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePeeringRequest(m.PeeringRequest, fromID, fromAddr) {
			s.handlePeeringRequest(m.PeeringRequest, fromID, fromAddr, fromKey, pkt.GetData())
		}

	// PeeringResponse
	case *pb.MessageWrapper_PeeringResponse:
		s.log.Debugw("handle "+m.PeeringResponse.Name(), "id", fromID, "addr", fromAddr)
		s.validatePeeringResponse(m.PeeringResponse, fromID, fromAddr)
		// PeeringResponse messages are handled in the handleReply function of the validation

	// PeeringDrop
	case *pb.MessageWrapper_PeeringDrop:
		s.log.Debugw("handle "+m.PeeringDrop.Name(), "id", fromID, "addr", fromAddr)
		if s.validatePeeringDrop(m.PeeringDrop, fromID, fromAddr) {
			s.handlePeeringDrop(m.PeeringDrop, fromID, fromAddr)
		}

	default:
		panic(fmt.Sprintf("invalid message type: %T", w.GetMessage()))
	}

	return nil
}

func decode(pkt *pb.Packet) (*pb.MessageWrapper, peer.PublicKey, error) {
	key, err := peer.RecoverKeyFromSignedData(pkt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid packet")
	}

	wrapper := &pb.MessageWrapper{}
	if err := proto.Unmarshal(pkt.GetData(), wrapper); err != nil {
		return nil, nil, errors.Wrap(err, "invalid message")
	}

	return wrapper, key, nil
}
