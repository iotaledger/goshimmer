package server

import (
	"time"

	"github.com/wollac/autopeering/peer"
)

const (
	packetExpiration = 20 * time.Second
	peerExpiration   = 12 * time.Hour
)

// Protocol provides a basis for server protocols handling incoming messages.
type Protocol struct {
	Local  *peer.Local // local peer that runs the protocol
	Sender Sender      // interface to send own requests
}

// LocalAddr returns the local network address in string form.
func (p *Protocol) LocalAddr() string {
	return p.Sender.LocalAddr()
}

// Send sends the data to the given peer.
func (p *Protocol) Send(to *peer.Peer, data []byte) {
	p.Sender.Send(to.Address(), data)
}

// SendExpectingReply sends request data to a peer and expects a response of the given type.
// On an incoming matching request the callback is executed to perform additional verification steps.
func (p *Protocol) SendExpectingReply(to *peer.Peer, data []byte, replyType MType, callback func(interface{}) bool) <-chan error {
	return p.Sender.SendExpectingReply(to.Address(), to.ID(), data, replyType, callback)
}

// IsVerified checks whether the given peer has recently been verified a recent enough endpoint proof.
func (p *Protocol) IsVerified(peer *peer.Peer) bool {
	return time.Since(p.Local.Database().LastPong(peer.ID(), peer.Address())) < peerExpiration
}

// HasVerified returns whether the given peer has recently verified the local peer.
func (p *Protocol) HasVerified(peer *peer.Peer) bool {
	return time.Since(p.Local.Database().LastPing(peer.ID(), peer.Address())) < peerExpiration
}

// IsExpired checks whether the given UNIX time stamp is too far in the past.
func (p *Protocol) IsExpired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= packetExpiration
}
