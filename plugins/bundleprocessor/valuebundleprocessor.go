package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/workerpool"
	"github.com/iotaledger/iota.go/curl"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/trinary"
)

var valueBundleProcessorWorkerPool = workerpool.New(func(task workerpool.Task) {
	if err := ProcessSolidValueBundle(task.Param(0).(*bundle.Bundle), task.Param(1).([]*value_transaction.ValueTransaction)); err != nil {
		Events.Error.Trigger(err)
	}

	task.Return(nil)
}, workerpool.WorkerCount(WORKER_COUNT), workerpool.QueueSize(2*WORKER_COUNT))

func ProcessSolidValueBundle(bundle *bundle.Bundle, bundleTransactions []*value_transaction.ValueTransaction) errors.IdentifiableError {
	bundle.SetBundleEssenceHash(CalculateBundleHash(bundleTransactions))

	Events.BundleSolid.Trigger(bundle, bundleTransactions)

	return nil
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

	bundleHash, err := curl.HashTrits(concatenatedBundleEssences)
	if err != nil {
		panic(err)
	}
	return trinary.MustTritsToTrytes(bundleHash)
}

func ValidateSignatures(bundleHash trinary.Hash, txs []*value_transaction.ValueTransaction) (bool, error) {
	for i, tx := range txs {
		// ignore all non-input transactions
		if tx.GetValue() >= 0 {
			continue
		}

		address := tx.GetAddress()

		// it is unknown how many fragments there will be
		fragments := []trinary.Trytes{tx.GetSignatureMessageFragment()}

		// each consecutive meta transaction with the same address contains another signature fragment
		for j := i; j < len(txs)-1; j++ {
			otherTx := txs[j+1]
			if otherTx.GetValue() != 0 || otherTx.GetAddress() != address {
				break
			}

			fragments = append(fragments, otherTx.GetSignatureMessageFragment())
		}

		// validate all the fragments against the address using Kerl
		valid, err := signing.ValidateSignatures(address, fragments, bundleHash)
		if err != nil {
			return false, err
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}
