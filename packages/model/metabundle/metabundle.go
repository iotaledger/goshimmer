package metabundle

import (
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

type MetaBundle struct {
	hash              ternary.Trytes
	transactionHashes []ternary.Trytes
}

func New(transactions []*meta_transaction.MetaTransaction) (result *MetaBundle) {
	result = &MetaBundle{
		hash: CalculateBundleHash(transactions),
	}

	return
}

func (bundle *MetaBundle) GetTransactionHashes() []ternary.Trytes {

}

func (bundle *MetaBundle) GetHash() {

}

func CalculateBundleHash(transactions []*meta_transaction.MetaTransaction) ternary.Trytes {
	return ternary.Trytes("A")
}
