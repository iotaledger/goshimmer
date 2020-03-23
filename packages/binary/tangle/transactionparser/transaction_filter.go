package transactionparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

type TransactionFilter interface {
	Filter(tx *message.Transaction, peer *peer.Peer)
	OnAccept(callback func(tx *message.Transaction, peer *peer.Peer))
	OnReject(callback func(tx *message.Transaction, err error, peer *peer.Peer))
	Shutdown()
}
