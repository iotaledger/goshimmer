package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/iota.go/trinary"
)

type tangleAPI interface {
	GetTransaction(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError)
	GetTransactionMetadata(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *transactionmetadata.TransactionMetadata) (result *transactionmetadata.TransactionMetadata, err errors.IdentifiableError)
}

type tangleStore struct{}

func (tangleStore) GetTransaction(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
	return tangle.GetTransaction(transactionHash)
}

func (tangleStore) GetTransactionMetadata(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *transactionmetadata.TransactionMetadata) (result *transactionmetadata.TransactionMetadata, err errors.IdentifiableError) {
	return tangle.GetTransactionMetadata(transactionHash)
}
