package transactionparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
)

type BytesFilter interface {
	Filter(bytes []byte, peer *peer.Peer)
	OnAccept(callback func(bytes []byte, peer *peer.Peer))
	OnReject(callback func(bytes []byte, err error, peer *peer.Peer))
	Shutdown()
}
