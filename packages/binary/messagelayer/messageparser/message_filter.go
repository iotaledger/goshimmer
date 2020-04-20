package messageparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// MessageFilter filters based on messages and peers.
type MessageFilter interface {
	// Filter filters up on the given message and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(msg *message.Message, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(msg *message.Message, peer *peer.Peer))
	// OnAccept registers the given callback as the rejection function of the filter.
	OnReject(callback func(msg *message.Message, err error, peer *peer.Peer))
	// Shutdown shuts down the filter.
	Shutdown()
}
