package discover

import (
	"container/list"
	"crypto/sha256"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
)

// Errors
var (
	errTimeout = errors.New("RPC timeout")
	errClosed  = errors.New("socket closed")
)

const (
	respTimeout = 500 * time.Millisecond
)

type protocol struct {
	trans Transport
	priv  *identity.PrivateIdentity

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{}
}

type replyMatcher struct {
	// these fields must match in the reply
	from   net.Addr
	fromID nodeId
	mtype  pb.MessageType

	// time when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives. If it returns matched == true, the
	// reply was acceptable. The second return value indicates whether the callback should
	// be removed from the pending reply queue. If it returns false, the reply is considered
	// incomplete and the callback will be invoked again for the next matching reply.
	callback replyMatchFunc

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout.
	errc chan error

	// reply contains the most recent reply.
	reply pb.Message
}

type replyMatchFunc func(interface{}) (matched bool, requestDone bool)

type nodeId = string

// reply is a reply packet from a certain node.
type reply struct {
	from    net.Addr
	fromID  nodeId
	message pb.Message

	// a matching request is indicated via this channel
	matched chan<- bool
}

func Listen(t Transport, priv *identity.PrivateIdentity) (*protocol, error) {
	p := &protocol{
		trans:           t,
		priv:            priv,
		addReplyMatcher: make(chan *replyMatcher),
		gotreply:        make(chan reply),
		closing:         make(chan struct{}),
	}

	p.wg.Add(2)
	go p.replyLoop()
	go p.readLoop()

	return p, nil
}

func (p *protocol) Close() {
	p.closeOnce.Do(func() {
		close(p.closing)
		p.trans.Close()
		p.wg.Wait()
	})
}

// Loop checking for matching replies.
func (p *protocol) replyLoop() {
	defer p.wg.Done()

	var (
		mlist   = list.New()
		timeout = time.NewTimer(0)
	)

	<-timeout.C // ignore first timeout
	defer timeout.Stop()

	for {

		// Set the timer so that it fires when the next reply expires
		if el := mlist.Front(); el != nil {
			// the first element always has the closest deadline
			m := el.Value.(*replyMatcher)
			dist := m.deadline.Sub(time.Now())
			timeout.Reset(dist)
		} else {
			timeout.Stop()
		}

		select {

		// add a new matcher to the list
		case p := <-p.addReplyMatcher:
			p.deadline = time.Now().Add(respTimeout)
			mlist.PushBack(p)

		// on reply received, check all matchers for fits
		case r := <-p.gotreply:
			var matched bool
			rtype := r.message.Type()
			for el := mlist.Front(); el != nil; el = el.Next() {
				m := el.Value.(*replyMatcher)
				if m.fromID == r.fromID && m.mtype == rtype && m.from.String() == r.from.String() {
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
		case <-p.closing:
			for el := mlist.Front(); el != nil; el = el.Next() {
				el.Value.(*replyMatcher).errc <- errClosed
			}
			return

		}
	}
}

func (p *protocol) expectReply(from net.Addr, fromID nodeId, mtype pb.MessageType, callback replyMatchFunc) *replyMatcher {
	ch := make(chan error, 1)
	m := &replyMatcher{from: from, fromID: fromID, mtype: mtype, callback: callback, errc: ch}
	select {
	case p.addReplyMatcher <- m:
	case <-p.closing:
		ch <- errClosed
	}
	return m
}

// Process a reply message. Returns whether a matching request could be found.
func (p *protocol) handleReply(from net.Addr, fromID nodeId, message pb.Message) bool {
	matched := make(chan bool, 1)
	select {
	case p.gotreply <- reply{from, fromID, message, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-p.closing:
		return false
	}
}

func (p *protocol) send(to net.Addr, toid nodeId, msg pb.Message) error {
	packet, err := encode(p.priv, msg)
	if err != nil {
		return err
	}
	return p.trans.Write(packet, to)
}

func encode(priv *identity.PrivateIdentity, message pb.Message) (*pb.Packet, error) {
	// wrap the message before marshalling
	data, err := proto.Marshal(message.Wrapper())
	if err != nil {
		return nil, errors.Wrap(err, "encode")
	}
	sig := priv.Sign(data)

	packet := &pb.Packet{
		PublicKey: priv.PublicKey,
		Signature: sig,
		Data:      data,
	}
	return packet, nil
}

func (p *protocol) readLoop() {
	defer p.wg.Done()

	pkt := &pb.Packet{}
	for {
		from, err := p.trans.Read(pkt)
		// exit when the connection is closed
		if err == io.EOF {
			return
		} else if err != nil {
			log.Println("Read error", err)
			continue
		}
		p.handlePacket(from, pkt)
	}
}

func (p *protocol) handlePacket(from net.Addr, pkt *pb.Packet) error {
	w, issuer, err := decode(pkt)
	if err != nil {
		log.Println("Bad packet", from, err)
		return err
	}
	fromID := issuer.StringId

	switch m := w.GetMessage().(type) {

	// Ping
	case *pb.MessageWrapper_Ping:
		if p.verifyPing(m.Ping, from, fromID) {
			p.handlePing(m.Ping, from, fromID)
		}

	// Pong
	case *pb.MessageWrapper_Pong:
		if p.verifyPong(m.Pong, from, fromID) {
			p.handlePong(m.Pong, from, fromID)
		}

	default:
		panic("invalid message type")
	}

	return nil
}

func decode(packet *pb.Packet) (*pb.MessageWrapper, *identity.Identity, error) {
	issuer, err := identity.NewIdentity(packet.GetPublicKey())
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid identity")
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

// TODO: this is not nice, consider moving away from net.Addr
func makeEndpoint(addr net.Addr) *pb.RpcEndpoint {
	ip, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	portNum, _ := strconv.ParseUint(port, 10, 32)
	return &pb.RpcEndpoint{Ip: ip, Port: uint32(portNum)}
}

// ------ Packet Handlers ------

func (p *protocol) verifyPing(ping *pb.Ping, from net.Addr, fromId nodeId) bool {
	// check to
	self := makeEndpoint(p.trans.LocalAddr())
	if !proto.Equal(ping.GetTo(), self) {
		return false
	}
	// check from
	if !proto.Equal(ping.GetFrom(), makeEndpoint(from)) {
		return false
	}
	return true
}

func (p *protocol) handlePing(ping *pb.Ping, from net.Addr, fromId nodeId) {
	// Reply with pong
	data, err := proto.Marshal(ping) // TODO: keep the raw data and pass it here
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(data)
	pong := &pb.Pong{To: makeEndpoint(from), PingHash: hash[:]}
	p.send(from, fromId, pong)
}

func (p *protocol) verifyPong(pong *pb.Pong, from net.Addr, fromId nodeId) bool {
	// there must be a ping waiting for this pong as a reply
	return p.handleReply(from, fromId, pong)
}

func (p *protocol) handlePong(pong *pb.Pong, from net.Addr, fromId nodeId) {
	// TODO: add as peer
}
