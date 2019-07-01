package fcob

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

type tangleAPI interface {
	GetTransaction(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError)
	GetTransactionMetadata(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *tangle.TransactionMetadata) (result *tangle.TransactionMetadata, err errors.IdentifiableError)
}

type tangleStore struct{}

func (tangleStore) GetTransaction(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
	return tangle.GetTransaction(transactionHash)
}

func (tangleStore) GetTransactionMetadata(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *tangle.TransactionMetadata) (result *tangle.TransactionMetadata, err errors.IdentifiableError) {
	return tangle.GetTransactionMetadata(transactionHash)
}
