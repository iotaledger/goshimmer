package server

import (
	"container/list"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	pb "github.com/iotaledger/goshimmer/packages/autopeering/server/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// ResponseTimeout specifies the time limit after which a response must have been received.
	ResponseTimeout = 500 * time.Millisecond
)

// Server offers the functionality to start a server that handles requests and responses from peers.
type Server struct {
	local    *peer.Local
	trans    transport.Transport
	handlers []Handler
	log      *zap.SugaredLogger
	network  string
	address  string

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	replyReceived   chan reply
	closing         chan struct{} // if this channel gets closed all pending waits should terminate
}

// a replyMatcher stores the information required to identify and react to an expected replay.
type replyMatcher struct {
	// fromAddr must match the sender of the reply
	fromAddr string
	// fromID must match the sender ID
	fromID peer.ID
	// mtype must match the type of the reply
	mtype MType

	// deadline when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives
	// If it returns true, the reply is acceptable.
	callback func(msg interface{}) bool

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout
	errc chan error
}

// reply is a reply packet from a certain peer
type reply struct {
	fromAddr       string
	fromID         peer.ID
	mtype          MType
	msg            interface{} // the actual reply message
	matchedRequest chan<- bool // a matching request is indicated via this channel
}

// Listen starts a new peer server using the given transport layer for communication.
// Sent data is signed using the identity of the local peer,
// received data with a valid peer signature is handled according to the provided Handler.
func Listen(local *peer.Local, t transport.Transport, log *zap.SugaredLogger, h ...Handler) *Server {
	srv := &Server{
		local:           local,
		trans:           t,
		handlers:        h,
		log:             log,
		network:         local.Network(),
		address:         local.Address(),
		addReplyMatcher: make(chan *replyMatcher),
		replyReceived:   make(chan reply),
		closing:         make(chan struct{}),
	}

	srv.wg.Add(2)
	go srv.replyLoop()
	go srv.readLoop()

	return srv
}

// Close shuts down the server.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.closing)
		s.trans.Close()
		s.wg.Wait()
	})
}

// Local returns the the local peer.
func (s *Server) Local() *peer.Local {
	return s.local
}

// LocalNetwork returns the network of the local peer.
func (s *Server) LocalNetwork() string {
	return s.network
}

// LocalAddr returns the address of the local peer in string form.
func (s *Server) LocalAddr() string {
	return s.address
}

// Send sends a message to the given address
func (s *Server) Send(toAddr string, data []byte) {
	pkt := s.encode(data)
	s.write(toAddr, pkt)
}

// SendExpectingReply sends a message to the given address and tells the Server
// to expect a reply message with the given specifications.
// If eventually nil is returned, a matching message was received.
func (s *Server) SendExpectingReply(toAddr string, toID peer.ID, data []byte, replyType MType, callback func(interface{}) bool) <-chan error {
	errc := s.expectReply(toAddr, toID, replyType, callback)
	s.Send(toAddr, data)

	return errc
}

// expectReply tells the Server to expect a reply message with the given specifications.
// If eventually nil is returned, a matching message was received.
func (s *Server) expectReply(fromAddr string, fromID peer.ID, mtype MType, callback func(interface{}) bool) <-chan error {
	ch := make(chan error, 1)
	m := &replyMatcher{fromAddr: fromAddr, fromID: fromID, mtype: mtype, callback: callback, errc: ch}
	select {
	case s.addReplyMatcher <- m:
	case <-s.closing:
		ch <- ErrClosed
	}
	return ch
}

// IsExpectedReply checks whether the given Message matches an expected reply added with SendExpectingReply.
func (s *Server) IsExpectedReply(fromAddr string, fromID peer.ID, mtype MType, msg interface{}) bool {
	matched := make(chan bool, 1)
	select {
	case s.replyReceived <- reply{fromAddr, fromID, mtype, msg, matched}:
		// wait for matcher and return whether a matching request was found
		return <-matched
	case <-s.closing:
		return false
	}
}

// Loop checking for matching replies.
func (s *Server) replyLoop() {
	defer s.wg.Done()

	var (
		matcherList = list.New()
		timeout     = time.NewTimer(0)
	)
	defer timeout.Stop()

	<-timeout.C // ignore first timeout

	for {

		// Set the timer so that it fires when the next reply expires
		if e := matcherList.Front(); e != nil {
			// the first element always has the closest deadline
			m := e.Value.(*replyMatcher)
			timeout.Reset(time.Until(m.deadline))
		} else {
			timeout.Stop()
		}

		select {

		// add a new matcher to the list
		case s := <-s.addReplyMatcher:
			s.deadline = time.Now().Add(ResponseTimeout)
			matcherList.PushBack(s)

		// on reply received, check all matchers for fits
		case r := <-s.replyReceived:
			matched := false
			for e := matcherList.Front(); e != nil; e = e.Next() {
				m := e.Value.(*replyMatcher)
				if m.mtype == r.mtype && m.fromID == r.fromID && m.fromAddr == r.fromAddr {
					if m.callback(r.msg) {
						// request has been matched
						matched = true
						m.errc <- nil
						matcherList.Remove(e)
					}
				}
			}
			r.matchedRequest <- matched

		// on timeout, check for expired matchers
		case <-timeout.C:
			now := time.Now()

			// notify and remove any expired matchers
			for e := matcherList.Front(); e != nil; e = e.Next() {
				m := e.Value.(*replyMatcher)
				if now.After(m.deadline) || now.Equal(m.deadline) {
					m.errc <- ErrTimeout
					matcherList.Remove(e)
				}
			}

		// on close, notice all the matchers
		case <-s.closing:
			for e := matcherList.Front(); e != nil; e = e.Next() {
				e.Value.(*replyMatcher).errc <- ErrClosed
			}
			return

		}
	}
}

func (s *Server) write(toAddr string, pkt *pb.Packet) {
	b, err := proto.Marshal(pkt)
	if err != nil {
		s.log.Error("marshal error", "err", err)
		return
	}
	if l := len(b); l > transport.MaxPacketSize {
		s.log.Error("packet too large", "size", l, "max", transport.MaxPacketSize)
		return
	}

	err = s.trans.WriteTo(b, toAddr)
	if err != nil {
		s.log.Debugw("failed to write packet", "addr", toAddr, "err", err)
	}
}

// encodes a message as a data packet that can be written.
func (s *Server) encode(data []byte) *pb.Packet {
	if len(data) == 0 {
		panic("server: no data")
	}

	return &pb.Packet{
		PublicKey: s.local.PublicKey(),
		Signature: s.local.Sign(data),
		Data:      append([]byte(nil), data...),
	}
}

func (s *Server) readLoop() {
	defer s.wg.Done()

	for {
		b, fromAddr, err := s.trans.ReadFrom()
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			s.log.Debugw("temporary read error", "err", err)
			continue
		}
		if err != nil {
			// return from the loop on all other errors
			if err != io.EOF {
				s.log.Warnw("read error", "err", err)
			}
			s.log.Debug("reading stopped")
			return
		}

		pkt := new(pb.Packet)
		if err := proto.Unmarshal(b, pkt); err != nil {
			s.log.Debugw("bad packet", "from", fromAddr, "err", err)
			continue
		}
		if err := s.handlePacket(pkt, fromAddr); err != nil {
			s.log.Debugw("failed to handle packet", "from", fromAddr, "err", err)
		}
	}
}

func (s *Server) handlePacket(pkt *pb.Packet, fromAddr string) error {
	data, key, err := decode(pkt)
	if err != nil {
		return errors.Wrap(err, "invalid packet")
	}

	fromID := key.ID()
	for _, handler := range s.handlers {
		ok, err := handler.HandleMessage(s, fromAddr, fromID, key, data)
		if ok {
			return err
		}
	}
	return ErrInvalidMessage
}

func decode(pkt *pb.Packet) ([]byte, peer.PublicKey, error) {
	if len(pkt.GetData()) == 0 {
		return nil, nil, ErrNoMessage
	}

	key, err := peer.RecoverKeyFromSignedData(pkt)
	if err != nil {
		return nil, nil, err
	}
	return pkt.GetData(), key, nil
}
