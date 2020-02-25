package transactionparser

import (
	"github.com/iotaledger/hive.go/autopeering/peer"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

type TransactionFilter interface {
	Filter(tx *transaction.Transaction, peer *peer.Peer)
	OnAccept(callback func(tx *transaction.Transaction, peer *peer.Peer))
	OnReject(callback func(tx *transaction.Transaction, err error, peer *peer.Peer))
	Shutdown()
}
