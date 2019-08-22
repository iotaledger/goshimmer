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
	"github.com/wollac/autopeering/transport"
)

const (
	// VersionNum is the expected version number of this protocol.
	VersionNum = 0

	respTimeout = 500 * time.Millisecond
)

type protocol struct {
	trans   transport.Transport
	priv    *identity.PrivateIdentity
	address string

	closeOnce sync.Once
	wg        sync.WaitGroup

	addReplyMatcher chan *replyMatcher
	gotreply        chan reply
	closing         chan struct{}
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

type nodeID = string

// reply is a reply packet from a certain node.
type reply struct {
	fromAddr string
	fromID   nodeID
	message  pb.Message

	// a matching request is indicated via this channel
	matched chan<- bool
}

// Listen starts a new peer discovery server using the given transport layer for communication.
func Listen(t transport.Transport, priv *identity.PrivateIdentity) (*protocol, error) {
	p := &protocol{
		trans:           t,
		priv:            priv,
		address:         t.LocalAddr(),
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

// LocalID returns the private idenity of the local node.
func (p *protocol) LocalID() *identity.PrivateIdentity {
	return p.priv
}

// localAddr returns the address of the local node in string form.
func (p *protocol) localAddr() string {
	return p.address
}

// ping sends a ping message to the given node and waits for a reply.
func (p *protocol) ping(toAddr string, toID nodeID) error {
	return <-p.sendPing(toAddr, toID, nil)
}

// sendPing pings the given node and invokes the callback when the reply arrives.
func (p *protocol) sendPing(toAddr string, toID nodeID, callback func()) <-chan error {
	// create the ping package
	ping := &pb.Ping{
		Version: VersionNum,
		From:    p.localAddr(),
		To:      toAddr,
	}
	pkt := encode(p.priv, ping)
	// compute the message hash
	hash := packetHash(pkt.GetData())

	// Add a matcher for the reply to the pending reply queue. Pongs are matched if they
	// reference the ping we're about to send.
	errc := p.expectReply(toAddr, toID, pb.MPong, func(m pb.Message) (matched bool, requestDone bool) {
		matched = bytes.Equal(m.(*pb.Pong).GetPingHash(), hash)
		if matched && callback != nil {
			callback()
		}
		return matched, matched
	})

	// send the ping
	p.write(toAddr, toID, ping.Name(), pkt)

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
			timeout.Reset(time.Until(m.deadline))
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
				if m.mtype == rtype && m.fromID == r.fromID && m.fromAddr == r.fromAddr {
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
func (p *protocol) expectReply(fromAddr string, fromID nodeID, mtype pb.MType, callback replyMatchFunc) <-chan error {
	ch := make(chan error, 1)
	m := &replyMatcher{fromAddr: fromAddr, fromID: fromID, mtype: mtype, callback: callback, errc: ch}
	select {
	case p.addReplyMatcher <- m:
	case <-p.closing:
		ch <- errClosed
	}
	return ch
}

// Process a reply message and returns whether a matching request could be found.
func (p *protocol) handleReply(fromAddr string, fromID nodeID, message pb.Message) bool {
	matched := make(chan bool, 1)
	select {
	case p.gotreply <- reply{fromAddr, fromID, message, matched}:
		// wait for matcher and return whether it could be matched
		return <-matched
	case <-p.closing:
		return false
	}
}

func (p *protocol) send(toAddr string, toID nodeID, msg pb.Message) {
	pkt := encode(p.priv, msg)
	p.write(toAddr, toID, msg.Name(), pkt)
}

func (p *protocol) write(toAddr string, toID nodeID, mName string, pkt *pb.Packet) {
	log.Println("write", mName, toID)
	if err := p.trans.WriteTo(pkt, toAddr); err != nil {
		log.Println("write", "error", err)
	}
}

func encode(priv *identity.PrivateIdentity, message pb.Message) *pb.Packet {
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

func (p *protocol) readLoop() {
	defer p.wg.Done()

	for {
		pkt, fromAddr, err := p.trans.ReadFrom()
		// exit when the connection is closed
		if err == io.EOF {
			log.Println("connection closed")
			return
		} else if err != nil {
			log.Println("read error", err)
			continue
		}

		if err := p.handlePacket(fromAddr, pkt); err != nil {
			log.Println("packet error", err)
		}
	}
}

func (p *protocol) handlePacket(fromAddr string, pkt *pb.Packet) error {
	w, issuer, err := decode(pkt)
	if err != nil {
		return err
	}
	fromID := issuer.StringID

	switch m := w.GetMessage().(type) {

	// Ping
	case *pb.MessageWrapper_Ping:
		if p.verifyPing(m.Ping, fromAddr, fromID) {
			p.handlePing(m.Ping, fromAddr, fromID, pkt.GetData())
		}

	// Pong
	case *pb.MessageWrapper_Pong:
		if p.verifyPong(m.Pong, fromAddr, fromID) {
			p.handlePong(m.Pong, fromAddr, fromID)
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

// ------ Packet Handlers ------

func (p *protocol) verifyPing(ping *pb.Ping, fromAddr string, fromID nodeID) bool {
	// check version number
	if ping.GetVersion() != VersionNum {
		log.Println("Ping", "invalid version:", ping.GetTo())
		return false
	}
	// check that To matches the local address
	if ping.GetTo() != p.localAddr() {
		log.Println("Ping", "invalid to:", ping.GetTo())
		return false
	}
	// check fromAddr
	if ping.GetFrom() != fromAddr {
		log.Println("Ping", "invalid from:", ping.GetFrom())
		return false
	}
	return true
}

func (p *protocol) handlePing(ping *pb.Ping, fromAddr string, fromID nodeID, rawData []byte) {
	log.Println("received", ping.Name(), fromID)

	pong := &pb.Pong{To: fromAddr, PingHash: packetHash(rawData)}
	p.send(fromAddr, fromID, pong)
}

func (p *protocol) verifyPong(pong *pb.Pong, fromAddr string, fromID nodeID) bool {
	// check that To matches the local address
	if pong.GetTo() != p.localAddr() {
		log.Println("Pong", "invalid to:", pong.GetTo())
		return false
	}
	// there must be a ping waiting for this pong as a reply
	if !p.handleReply(fromAddr, fromID, pong) {
		log.Println("Pong", "no matching request")
		return false
	}
	return true
}

func (p *protocol) handlePong(pong *pb.Pong, fromAddr string, fromID nodeID) {
	log.Println("received", pong.Name(), fromID)

	// TODO: add as peer
}
