package discover

import (
	"bytes"
	"container/list"
	"io"
	"log"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/wollac/autopeering/identity"
	pb "github.com/wollac/autopeering/proto"
	trans "github.com/wollac/autopeering/transport"
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
	trans trans.Transport
	priv  *identity.PrivateIdentity

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{}
}

type replyMatcher struct {
	// these fields must match in the reply
	from   *trans.Addr
	fromID nodeID
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

type nodeID = string

// reply is a reply packet from a certain node.
type reply struct {
	from    *trans.Addr
	fromID  nodeID
	message pb.Message

	// a matching request is indicated via this channel
	matched chan<- bool
}

func Listen(t trans.Transport, priv *identity.PrivateIdentity) (*protocol, error) {
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

func (p *protocol) OwnId() *identity.PrivateIdentity {
	return p.priv
}

// ping sends a ping message to the given node and waits for a reply.
func (p *protocol) ping(to *trans.Addr, toID nodeID) error {
	return <-p.sendPing(to, toID, nil)
}

// Sends a ping to the given node and invokes the callback when the reply arrives.
func (p *protocol) sendPing(to *trans.Addr, toID nodeID, callback func()) <-chan error {
	// create the ping package
	ping := &pb.Ping{
		Version: 0,
		From:    makeEndpoint(p.trans.LocalEndpoint()),
		To:      makeEndpoint(to),
	}
	packet, hash, err := encode(p.priv, ping)
	if err != nil {
		errc := make(chan error, 1)
		errc <- err
		return errc
	}

	// Add a matcher for the reply to the pending reply queue. Pongs are matched if they
	// reference the ping we're about to send.
	errc := p.expectReply(to, toID, pb.PONG, func(p interface{}) (matched bool, requestDone bool) {
		matched = bytes.Equal(p.(*pb.Pong).GetPingHash(), hash)
		if matched && callback != nil {
			callback()
		}
		return matched, matched
	})

	if err := p.trans.Write(packet, to); err != nil {
		errc := make(chan error, 1)
		errc <- err
		return errc
	}

	return errc
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
				if m.fromID == r.fromID && m.mtype == rtype && m.from.Equal(r.from) {
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

// Expects a reply message with the given specifications.
// If eventually nil is returned, a matching message was received.
func (p *protocol) expectReply(from *trans.Addr, fromID nodeID, mtype pb.MessageType, callback replyMatchFunc) <-chan error {
	ch := make(chan error, 1)
	m := &replyMatcher{from: from, fromID: fromID, mtype: mtype, callback: callback, errc: ch}
	select {
	case p.addReplyMatcher <- m:
	case <-p.closing:
		ch <- errClosed
	}
	return ch
}

// Process a reply message. Returns whether a matching request could be found.
func (p *protocol) handleReply(from *trans.Addr, fromID nodeID, message pb.Message) bool {
	matched := make(chan bool, 1)
	select {
	case p.gotreply <- reply{from, fromID, message, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-p.closing:
		return false
	}
}

func (p *protocol) send(to *trans.Addr, toID nodeID, msg pb.Message) error {
	packet, _, err := encode(p.priv, msg)
	if err != nil {
		return err
	}
	return p.trans.Write(packet, to)
}

func encode(priv *identity.PrivateIdentity, message pb.Message) (*pb.Packet, []byte, error) {
	// wrap the message before marshalling
	data, err := proto.Marshal(message.Wrapper())
	if err != nil {
		return nil, nil, errors.Wrap(err, "encode")
	}
	sig := priv.Sign(data)

	packet := &pb.Packet{
		PublicKey: priv.PublicKey,
		Signature: sig,
		Data:      data,
	}
	return packet, packetHash(data), nil
}

func (p *protocol) readLoop() {
	defer p.wg.Done()

	for {
		pkt, from, err := p.trans.Read()
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

func (p *protocol) handlePacket(from *trans.Addr, pkt *pb.Packet) error {
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

func makeEndpoint(addr *trans.Addr) *pb.RpcEndpoint {
	return &pb.RpcEndpoint{
		Ip:   addr.IP.String(),
		Port: uint32(addr.Port),
	}
}

func endpointEquals(a *trans.Addr, e *pb.RpcEndpoint) bool {
	return uint32(a.Port) != e.GetPort() || a.IP.String() != e.GetIp()
}

// ------ Packet Handlers ------

func (p *protocol) verifyPing(ping *pb.Ping, from *trans.Addr, fromID nodeID) bool {
	// check to
	if !endpointEquals(p.trans.LocalEndpoint(), ping.GetTo()) {
		return false
	}
	// check from
	if !endpointEquals(from, ping.GetFrom()) {
		return false
	}
	return true
}

func (p *protocol) handlePing(ping *pb.Ping, from *trans.Addr, fromID nodeID) {
	// Reply with pong
	data, err := proto.Marshal(ping) // TODO: keep the raw data and pass it here
	if err != nil {
		panic(err)
	}
	pong := &pb.Pong{To: makeEndpoint(from), PingHash: packetHash(data)}
	p.send(from, fromID, pong)
}

func (p *protocol) verifyPong(pong *pb.Pong, from *trans.Addr, fromID nodeID) bool {
	// there must be a ping waiting for this pong as a reply
	return p.handleReply(from, fromID, pong)
}

func (p *protocol) handlePong(pong *pb.Pong, from *trans.Addr, fromID nodeID) {
	// TODO: add as peer
}
