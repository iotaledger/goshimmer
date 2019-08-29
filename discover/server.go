package discover

import (
	"bytes"
	"container/list"
	"io"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/id"
	pb "github.com/wollac/autopeering/proto"
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

type Server struct {
	trans   transport.Transport
	mgr     *manager
	priv    *id.Private
	log     *zap.SugaredLogger
	address string

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{} // if this channel gets closed all pending waits should terminate
}

// nodeID is an alias for the public key of the node.
// For efficiency reasons, we don't use id.Identity directly.
type nodeID []byte

func getNodeID(id *id.Identity) nodeID {
	return nodeID(id.ID())
}

func (id nodeID) equals(x nodeID) bool {
	return bytes.Equal(id, x)
}

// replyMatchFunc is the type of the matcher callback. If it returns matched, the
// reply was acceptable. If requestDone is false, the reply is considered
// incomplete and the callback will be invoked again for the next matching reply.
type replyMatchFunc func(pb.Message) (matched bool, requestDone bool)

type replyMatcher struct {
	// fromAddr must match the sender of the reply
	fromAddr string
	// fromID must match the sender ID
	fromID nodeID
	// mtype must match the type of the reply
	mtype pb.MType

	// deadline when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives
	callback replyMatchFunc

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout
	errc chan error

	// reply contains the most recent reply
	reply pb.Message
}

// reply is a reply packet from a certain node.
type reply struct {
	fromAddr string
	fromID   nodeID
	message  pb.Message

	// a matching request is indicated via this channel
	matched chan<- bool
}

// Listen starts a new peer discovery server using the given transport layer for communication.
func Listen(t transport.Transport, cfg Config) (*Server, error) {
	s := &Server{
		trans:           t,
		priv:            cfg.ID,
		log:             cfg.Log,
		address:         t.LocalAddr(),
		addReplyMatcher: make(chan *replyMatcher),
		gotreply:        make(chan reply),
		closing:         make(chan struct{}),
	}
	s.mgr = newManager(s, cfg.Bootnodes, s.log.Named("mgr"))

	s.wg.Add(2)
	go s.replyLoop()
	go s.readLoop()

	return s, nil
}

func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.closing)
		s.mgr.close()
		s.trans.Close()
		s.wg.Wait()
	})
}

// LocalID returns the private idenity of the local node.
func (s *Server) LocalID() *id.Private {
	return s.priv
}

// LocalAddr returns the address of the local node in string form.
func (s *Server) LocalAddr() string {
	return s.address
}

// ping sends a ping message to the given node and waits for a reply.
func (s *Server) ping(peer *Peer) error {
	return <-s.sendPing(peer, nil)
}

// sendPing pings the given node and invokes the callback when the reply arrives.
func (s *Server) sendPing(peer *Peer, callback func()) <-chan error {
	toAddr := peer.Address
	toID := getNodeID(peer.Identity)

	// create the ping package
	ping := newPing(s.LocalAddr(), toAddr)
	pkt := encode(s.priv, ping)
	// compute the message hash
	hash := packetHash(pkt.GetData())

	// Add a matcher for the reply to the pending reply queue. Pongs are matched if they
	// reference the ping we're about to send.
	errc := s.expectReply(toAddr, toID, pb.MPong, func(m pb.Message) (bool, bool) {
		matched := bytes.Equal(m.(*pb.Pong).GetPingHash(), hash)
		if matched && callback != nil {
			callback()
		}
		return matched, matched
	})

	// send the ping
	s.write(toAddr, ping.Name(), pkt)

	return errc
}

func (s *Server) requestPeers(to *Peer) <-chan error {
	s.ensureBond(to)

	toID := getNodeID(to.Identity)
	toAddr := to.Address

	// create the request package
	req := newPeersRequest()
	pkt := encode(s.priv, req)
	// compute the message hash
	hash := packetHash(pkt.GetData())

	errc := s.expectReply(toAddr, toID, pb.MPeersResponse, func(m pb.Message) (bool, bool) {
		matched := bytes.Equal(m.(*pb.PeersResponse).GetReqHash(), hash)
		return matched, matched
	})

	// send the request
	s.write(toAddr, req.Name(), pkt)

	return errc
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
			var matched bool
			rtype := r.message.Type()
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*replyMatcher)
				if m.mtype == rtype && m.fromAddr == r.fromAddr && m.fromID.equals(r.fromID) {
					ok, requestDone := m.callback(r.message)
					matched = matched || ok

					if requestDone {
						m.reply = r.message
						m.errc <- nil
						mlist.Remove(el)
					}
				}
			}
			r.matched <- matched

		// on timeout, check for expired matchers
		case <-timeout.C:
			now := time.Now()

			// Notify and remove expired matchers
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
func (s *Server) expectReply(fromAddr string, fromID nodeID, mtype pb.MType, callback replyMatchFunc) <-chan error {
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
func (s *Server) handleReply(fromAddr string, fromID *id.Identity, message pb.Message) bool {
	matched := make(chan bool, 1)
	select {
	case s.gotreply <- reply{fromAddr, getNodeID(fromID), message, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-s.closing:
		return false
	}
}

func (s *Server) send(toAddr string, msg pb.Message) {
	pkt := encode(s.priv, msg)
	s.write(toAddr, msg.Name(), pkt)
}

func (s *Server) write(toAddr string, mName string, pkt *pb.Packet) {
	err := s.trans.WriteTo(pkt, toAddr)
	s.log.Debugw("write "+mName, "to", toAddr, "err", err)
}

func encode(priv *id.Private, message pb.Message) *pb.Packet {
	// wrap the message before marshaling
	data, err := proto.Marshal(message.Wrapper())
	if err != nil {
		panic("protobuf error: " + err.Error())
	}

	sig := priv.Sign(data)
	return &pb.Packet{
		PublicKey: priv.PublicKey,
		Signature: sig,
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

		if err := s.handlePacket(fromAddr, pkt); err != nil {
			s.log.Warnw("failed to handle packet", "from", fromAddr, "err", err)
		}
	}
}

func decode(packet *pb.Packet) (*pb.MessageWrapper, *id.Identity, error) {
	issuer, err := id.NewIdentity(packet.GetPublicKey())
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid id")
	}

	data := packet.GetData()
	if !issuer.VerifySignature(data, packet.GetSignature()) {
		return nil, nil, errors.Wrap(err, "invalid signature")
	}

	wrapper := &pb.MessageWrapper{}
	if err := proto.Unmarshal(data, wrapper); err != nil {
		return nil, nil, errors.Wrap(err, "invalid message data")
	}

	return wrapper, issuer, nil
}

// checkBond checks if the given node has a recent enough endpoint proof.
func (s *Server) checkBond(peer *Peer) bool {
	return time.Since(s.mgr.db.LastPong(peer)) < pongExpiration
}

// ensureBond solicits a ping from a node if we haven't seen a ping from it for a while.
func (s *Server) ensureBond(peer *Peer) {
	if time.Since(s.mgr.db.LastPing(peer)) >= pongExpiration {
		<-s.sendPing(peer, nil)
		// Wait for them to ping back and process our pong.
		time.Sleep(responseTimeout)
	}
}

// expired checks whether the given UNIX time stamp is too far in the past.
func expired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= packetExpiration
}
