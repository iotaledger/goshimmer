package messageparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
)

// BytesFilter filters based on byte slices and peers.
type BytesFilter interface {
	// Filter filters up on the given bytes and peer and calls the acceptance callback
	// if the input passes or the rejection callback if the input is rejected.
	Filter(bytes []byte, peer *peer.Peer)
	// OnAccept registers the given callback as the acceptance function of the filter.
	OnAccept(callback func(bytes []byte, peer *peer.Peer))
	// OnAccept registers the given callback as the rejection function of the filter.
	OnReject(callback func(bytes []byte, err error, peer *peer.Peer))
	// Shutdown shuts down the filter.
	Shutdown()
}
