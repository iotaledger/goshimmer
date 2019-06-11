package value_transaction

import (
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

type ValueTransaction struct {
	*meta_transaction.MetaTransaction

	data ternary.Trits
}

func New() (result *ValueTransaction) {
	result = &ValueTransaction{
		MetaTransaction: meta_transaction.New(),
	}

	result.data = result.MetaTransaction.GetData()

	return
}
