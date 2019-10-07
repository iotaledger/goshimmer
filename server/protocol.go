package server

import (
	"time"

	"github.com/wollac/autopeering/peer"
)

const (
	packetExpiration = 20 * time.Second
)

// Protocol provides a basis for server protocols handling incoming messages.
type Protocol struct {
	Sender Sender // interface to send own requests
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

// IsExpired checks whether the given UNIX time stamp is too far in the past.
func (p *Protocol) IsExpired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= packetExpiration
}
