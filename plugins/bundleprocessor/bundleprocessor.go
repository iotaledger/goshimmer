package bundleprocessor

import (
	"fmt"
	"runtime"

	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"
)

var workerPool = workerpool.New(func(task workerpool.Task) {
	if err := ProcessSolidBundleHead(task.Param(0).(*value_transaction.ValueTransaction)); err != nil {
		Events.Error.Trigger(err)
	}

	task.Return(nil)
}, workerpool.WorkerCount(WORKER_COUNT), workerpool.QueueSize(2*WORKER_COUNT))

var WORKER_COUNT = runtime.NumCPU()

func ProcessSolidBundleHead(headTransaction *value_transaction.ValueTransaction) errors.IdentifiableError {
	// only process the bundle if we didn't process it, yet
	_, err := tangle.GetBundle(headTransaction.GetHash(), func(headTransactionHash trinary.Trytes) (*bundle.Bundle, errors.IdentifiableError) {
		// abort if bundle syntax is wrong
		if !headTransaction.IsHead() {
			return nil, ErrProcessBundleFailed.Derive(errors.New("invalid parameter"), "transaction needs to be head of bundle")
		}

		// initialize event variables
		newBundle := bundle.New(headTransactionHash)
		bundleTransactions := make([]*value_transaction.ValueTransaction, 0)

		// iterate through trunk transactions until we reach the tail
		currentTransaction := headTransaction
		for {
			// abort if we reached a previous head
			if currentTransaction.IsHead() && currentTransaction != headTransaction {
				newBundle.SetTransactionHashes(mapTransactionsToTransactionHashes(bundleTransactions))

				Events.InvalidBundle.Trigger(newBundle, bundleTransactions)

				return nil, ErrProcessBundleFailed.Derive(errors.New("invalid bundle found"), "missing bundle tail")
			}

			// update bundle transactions
			bundleTransactions = append(bundleTransactions, currentTransaction)

			// retrieve & update metadata
			currentTransactionMetadata, dbErr := tangle.GetTransactionMetadata(currentTransaction.GetHash(), transactionmetadata.New)
			if dbErr != nil {
				return nil, ErrProcessBundleFailed.Derive(dbErr, "failed to retrieve transaction metadata")
			}
			currentTransactionMetadata.SetBundleHeadHash(headTransactionHash)

			// update value bundle flag
			if !newBundle.IsValueBundle() && currentTransaction.GetValue() < 0 {
				newBundle.SetValueBundle(true)
			}

			// if we are done -> trigger events
			if currentTransaction.IsTail() {
				newBundle.SetTransactionHashes(mapTransactionsToTransactionHashes(bundleTransactions))

				if newBundle.IsValueBundle() {
					valueBundleProcessorWorkerPool.Submit(newBundle, bundleTransactions)

					return newBundle, nil
				}

				Events.BundleSolid.Trigger(newBundle, bundleTransactions)

				return newBundle, nil
			}

			// try to iterate to next turn
			if nextTransaction, err := tangle.GetTransaction(currentTransaction.GetTrunkTransactionHash()); err != nil {
				return nil, ErrProcessBundleFailed.Derive(err, "failed to retrieve trunk while processing bundle")
			} else if nextTransaction == nil {
				fmt.Println(ErrProcessBundleFailed.Derive(errors.New("missing transaction "+currentTransaction.GetTrunkTransactionHash()), "failed to retrieve trunk while processing bundle"))
				return nil, ErrProcessBundleFailed.Derive(err, "failed to retrieve trunk while processing bundle")
			} else {
				currentTransaction = nextTransaction
			}
		}
	})

	return err
}

func mapTransactionsToTransactionHashes(transactions []*value_transaction.ValueTransaction) (result []trinary.Trytes) {
	result = make([]trinary.Trytes, len(transactions))
	for k, v := range transactions {
		result[k] = v.GetHash()
	}

	return
}
