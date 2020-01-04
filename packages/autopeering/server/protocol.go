package server

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
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

// LocalNetwork returns the name of the local network (for example, "tcp", "udp").
func (p *Protocol) LocalNetwork() string {
	return p.Sender.LocalNetwork()
}

// Send sends the data to the given peer.
func (p *Protocol) Send(to *peer.Peer, data []byte) {
	p.Sender.Send(to.Address(), data)
}

// SendExpectingReply sends request data to a peer and expects a response of the given type.
// On an incoming matching request the callback is executed to perform additional verification steps.
func (p *Protocol) SendExpectingReply(toAddr string, toID peer.ID, data []byte, replyType MType, callback func(interface{}) bool) <-chan error {
	return p.Sender.SendExpectingReply(toAddr, toID, data, replyType, callback)
}

// IsExpired checks whether the given UNIX time stamp is too far in the past.
func (p *Protocol) IsExpired(ts int64) bool {
	return time.Since(time.Unix(ts, 0)) >= packetExpiration
}
