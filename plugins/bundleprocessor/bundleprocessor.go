package bundleprocessor

import (
	"fmt"
	"runtime"

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

func ProcessSolidBundleHead(headTransaction *value_transaction.ValueTransaction) error {
	// only process the bundle if we didn't process it, yet
	_, err := tangle.GetBundle(headTransaction.GetHash(), func(headTransactionHash trinary.Trytes) (*bundle.Bundle, error) {
		// abort if bundle syntax is wrong
		if !headTransaction.IsHead() {
			return nil, fmt.Errorf("%w: transaction needs to be the head of the bundle", ErrProcessBundleFailed)
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
				return nil, fmt.Errorf("%w: missing bundle tail", ErrProcessBundleFailed)
			}

			// update bundle transactions
			bundleTransactions = append(bundleTransactions, currentTransaction)

			// retrieve & update metadata
			currentTransactionMetadata, err := tangle.GetTransactionMetadata(currentTransaction.GetHash(), transactionmetadata.New)
			if err != nil {
				return nil, fmt.Errorf("%w: failed to retrieve transaction metadata: %s", ErrProcessBundleFailed, err)
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
				return nil, fmt.Errorf("%w: failed to retrieve trunk while processing bundle: %s", ErrProcessBundleFailed, err)
			} else if nextTransaction == nil {
				err := fmt.Errorf("%w: failed to retrieve trunk while processing bundle: missing trunk transaction %s\n", ErrProcessBundleFailed, currentTransaction.GetTrunkTransactionHash())
				fmt.Println(err)
				return nil, err
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
