package server

import (
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	pb "github.com/iotaledger/autopeering-sim/server/proto"
	"github.com/iotaledger/autopeering-sim/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// ResponseTimeout specifies the time limit after which a response must have been received.
	ResponseTimeout = 500 * time.Millisecond
)

// Server offers the functionality to start a discovery server.
type Server struct {
	local    *peer.Local
	trans    transport.Transport
	handlers []Handler
	log      *zap.SugaredLogger
	address  string

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{} // if this channel gets closed all pending waits should terminate
}

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
	fromAddr string
	fromID   peer.ID
	mtype    MType
	msg      interface{}

	// a matching request is indicated via this channel
	matched chan<- bool
}

// Listen starts a new peer discovery server using the given transport layer for communication.
func Listen(local *peer.Local, t transport.Transport, log *zap.SugaredLogger, h ...Handler) *Server {
	srv := &Server{
		local:           local,
		trans:           t,
		handlers:        h,
		log:             log,
		address:         t.LocalAddr(),
		addReplyMatcher: make(chan *replyMatcher),
		gotreply:        make(chan reply),
		closing:         make(chan struct{}),
	}

	host, port, _ := net.SplitHostPort(srv.address)
	if host == "0.0.0.0" {
		srv.address = getMyIP() + ":" + port
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
func (s *Server) IsExpectedReply(from *peer.Peer, mtype MType, msg interface{}) bool {
	matched := make(chan bool, 1)
	select {
	case s.gotreply <- reply{from.Address(), from.ID(), mtype, msg, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-s.closing:
		return false
	}
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
			s.deadline = time.Now().Add(ResponseTimeout)
			mlist.PushBack(s)

		// on reply received, check all matchers for fits
		case r := <-s.gotreply:
			matched := false
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*replyMatcher)
				if m.mtype == r.mtype && m.fromID == r.fromID && m.fromAddr == r.fromAddr {
					if m.callback(r.msg) {
						matched = true

						// request is done
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
					m.errc <- ErrTimeout
					mlist.Remove(el)
				}
			}

		// on close, notice all the matchers
		case <-s.closing:
			for el := mlist.Front(); el != nil; el = el.Next() {
				el.Value.(*replyMatcher).errc <- ErrClosed
			}
			return

		}
	}
}

func (s *Server) write(toAddr string, pkt *pb.Packet) {
	err := s.trans.WriteTo(pkt, toAddr)
	s.log.Debugw("write packet", "to", toAddr, "err", err)
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
	data, key, err := decode(pkt)
	if err != nil {
		return errors.Wrap(err, "invalid packet")
	}

	from := peer.NewPeer(key, fromAddr)
	s.log.Debugw("handlePacket", "id", from.ID(), "addr", fromAddr)
	for _, handler := range s.handlers {
		ok, err := handler.HandleMessage(s, from, data)
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

func getMyIP() string {
	url := "https://api.ipify.org?format=text"
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s", ip)
}
