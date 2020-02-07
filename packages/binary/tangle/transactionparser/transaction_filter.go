package transactionparser

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

type TransactionFilter interface {
	Filter(tx *transaction.Transaction)
	OnAccept(callback func(tx *transaction.Transaction))
	OnReject(callback func(tx *transaction.Transaction))
	Shutdown()
}
