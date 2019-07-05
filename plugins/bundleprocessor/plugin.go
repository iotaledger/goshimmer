package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("Bundle Processor", configure)

func configure(plugin *node.Plugin) {
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		if tx.IsHead() {
			if _, err := ProcessSolidBundleHead(tx); err != nil {
				plugin.LogFailure(err.Error())
			}
		}
	}))
}

func ProcessSolidBundleHead(headTransaction *value_transaction.ValueTransaction) (*bundle.Bundle, errors.IdentifiableError) {
	// only process the bundle if we didn't process it, yet
	return tangle.GetBundle(headTransaction.GetHash(), func(headTransactionHash ternary.Trytes) (*bundle.Bundle, errors.IdentifiableError) {
		// abort if bundle syntax is wrong
		if !headTransaction.IsHead() {
			return nil, ErrProcessBundleFailed.Derive(errors.New("invalid parameter"), "transaction needs to be head of bundle")
		}

		// initialize result variables
		processedBundle := bundle.New(headTransactionHash)
		bundleTransactions := make([]*value_transaction.ValueTransaction, 0)

		// iterate through trunk transactions until we reach the tail
		currentTransaction := headTransaction
		for {
			// abort if we reached a previous head
			if currentTransaction.IsHead() && currentTransaction != headTransaction {
				processedBundle.SetTransactionHashes(mapTransactionsToTransactionHashes(bundleTransactions))

				Events.InvalidBundleReceived.Trigger(processedBundle, bundleTransactions)

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
			if !processedBundle.IsValueBundle() && currentTransaction.GetValue() != 0 {
				processedBundle.SetValueBundle(true)
			}

			// if we are done -> trigger events
			if currentTransaction.IsTail() {
				processedBundle.SetTransactionHashes(mapTransactionsToTransactionHashes(bundleTransactions))

				if processedBundle.IsValueBundle() {
					Events.ValueBundleReceived.Trigger(processedBundle, bundleTransactions)
				} else {
					Events.DataBundleReceived.Trigger(processedBundle, bundleTransactions)
				}

				return processedBundle, nil
			}

			// try to iterate to next turn
			if nextTransaction, err := tangle.GetTransaction(currentTransaction.GetTrunkTransactionHash()); err != nil {
				return nil, ErrProcessBundleFailed.Derive(err, "failed to retrieve trunk while processing bundle")
			} else {
				currentTransaction = nextTransaction
			}
		}
	})
}

func mapTransactionsToTransactionHashes(transactions []*value_transaction.ValueTransaction) []ternary.Trytes {
	result := make([]ternary.Trytes, len(transactions))

	for i, v := range transactions {
		result[i] = v.GetHash()
	}

	return result
}
