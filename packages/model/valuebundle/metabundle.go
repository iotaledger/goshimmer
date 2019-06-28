package valuebundle

import (
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

type MetaBundle struct {
	hash              ternary.Trytes
	transactionHashes []ternary.Trytes
}

func New(transactions []*value_transaction.ValueTransaction) (result *MetaBundle) {
	result = &MetaBundle{
		hash: CalculateBundleHash(transactions),
	}

	return
}

func (bundle *MetaBundle) GetTransactionHashes() []ternary.Trytes {
	return bundle.transactionHashes
}

func (bundle *MetaBundle) GetHash() ternary.Trytes {
	return bundle.hash
}

func CalculateBundleHash(transactions []*value_transaction.ValueTransaction) ternary.Trytes {
	//transactions[0].GetData()[value_transaction.SIGNATURE_MESSAGE_FRAGMENT_OFFSET]
	return ternary.Trytes("A")
}
