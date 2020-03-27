package transactionparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/message"
)

type TransactionFilter interface {
	Filter(tx *message.Message, peer *peer.Peer)
	OnAccept(callback func(tx *message.Message, peer *peer.Peer))
	OnReject(callback func(tx *message.Message, err error, peer *peer.Peer))
	Shutdown()
}
