package client

import (
	"github.com/iotaledger/goshimmer/packages/curl"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/iota.go/trinary"
)

type Bundle struct {
	essenceHash  trinary.Trytes
	transactions []*value_transaction.ValueTransaction
}

func (bundle *Bundle) GetEssenceHash() trinary.Trytes {
	return bundle.essenceHash
}

func (bundle *Bundle) GetTransactions() []*value_transaction.ValueTransaction {
	return bundle.transactions
}

func CalculateBundleHash(transactions []*value_transaction.ValueTransaction) trinary.Trytes {
	var lastInputAddress trinary.Trytes

	var concatenatedBundleEssences = make(trinary.Trits, len(transactions)*value_transaction.BUNDLE_ESSENCE_SIZE)
	for i, bundleTransaction := range transactions {
		if bundleTransaction.GetValue() <= 0 {
			lastInputAddress = bundleTransaction.GetAddress()
		}

		copy(concatenatedBundleEssences[value_transaction.BUNDLE_ESSENCE_SIZE*i:value_transaction.BUNDLE_ESSENCE_SIZE*(i+1)], bundleTransaction.GetBundleEssence(lastInputAddress != bundleTransaction.GetAddress()))
	}

	var bundleHash = make(trinary.Trits, 243)

	hasher := curl.NewCurl(243, 81)
	hasher.Absorb(concatenatedBundleEssences, 0, len(concatenatedBundleEssences))
	hasher.Squeeze(bundleHash, 0, 243)

	return trinary.MustTritsToTrytes(bundleHash)
}
