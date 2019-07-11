package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/curl"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/workerpool"
	"github.com/iotaledger/iota.go/trinary"
)

var valueBundleProcessorWorkerPool = workerpool.New(func(task workerpool.Task) {
	if err := ProcessSolidValueBundle(task.Param(0).(*bundle.Bundle), task.Param(1).([]*value_transaction.ValueTransaction)); err != nil {
		Events.Error.Trigger(err)
	}
}, workerpool.WorkerCount(WORKER_COUNT), workerpool.QueueSize(2*WORKER_COUNT))

func ProcessSolidValueBundle(bundle *bundle.Bundle, bundleTransactions []*value_transaction.ValueTransaction) errors.IdentifiableError {
	var concatenatedBundleEssences = make(trinary.Trits, len(bundleTransactions)*value_transaction.BUNDLE_ESSENCE_SIZE)
	for i, bundleTransaction := range bundleTransactions {
		copy(concatenatedBundleEssences[value_transaction.BUNDLE_ESSENCE_SIZE*i:value_transaction.BUNDLE_ESSENCE_SIZE*(i+1)], bundleTransaction.GetBundleEssence())
	}

	var resp = make(trinary.Trits, 243)

	hasher := curl.NewCurl(243, 81)
	hasher.Absorb(concatenatedBundleEssences, 0, len(concatenatedBundleEssences))
	hasher.Squeeze(resp, 0, 243)

	bundle.SetBundleEssenceHash(trinary.MustTritsToTrytes(resp))

	Events.BundleSolid.Trigger(bundle, bundleTransactions)

	return nil
}
