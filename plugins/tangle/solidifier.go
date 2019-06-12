package tangle

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

func configureSolidifier(plugin *node.Plugin) {
	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(rawTransaction *transaction.Transaction) {
		go processRawTransaction(plugin, rawTransaction)
	}))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Checks and updates the solid flag of a single transaction.
func checkSolidity(transaction *Transaction) (result bool, err errors.IdentifiableError) {
	// abort if transaction is solid already
	txMetadata, metaDataErr := transaction.GetMetaData()
	if metaDataErr != nil {
		err = metaDataErr

		return
	} else if txMetadata.GetSolid() {
		result = true

		return
	}

	// check solidity of branch transaction if it is not genesis
	if branchTransactionHash := transaction.GetBranchTransactionHash(); branchTransactionHash != TRANSACTION_NULL_HASH {
		// abort if branch transaction is missing
		if branchTransaction, branchErr := GetTransaction(branchTransactionHash); branchErr != nil {
			err = branchErr

			return
		} else if branchTransaction == nil {
			return
		} else if branchTransactionMetadata, branchErr := branchTransaction.GetMetaData(); branchErr != nil {
			err = branchErr

			return
		} else if !branchTransactionMetadata.GetSolid() {
			return
		}
	}

	// check solidity of branch transaction if it is not genesis
	if trunkTransactionHash := transaction.GetBranchTransactionHash(); trunkTransactionHash != TRANSACTION_NULL_HASH {
		if trunkTransaction, trunkErr := GetTransaction(trunkTransactionHash); trunkErr != nil {
			err = trunkErr

			return
		} else if trunkTransaction == nil {
			return
		} else if trunkTransactionMetadata, trunkErr := trunkTransaction.GetMetaData(); trunkErr != nil {
			err = trunkErr

			return
		} else if !trunkTransactionMetadata.GetSolid() {
			return
		}
	}

	// mark transaction as solid and trigger event
	if txMetadata.SetSolid(true) {
		Events.TransactionSolid.Trigger(transaction)
	}

	result = true

	return
}

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func IsSolid(transaction *Transaction) (bool, errors.IdentifiableError) {
	if isSolid, err := checkSolidity(transaction); err != nil {
		return false, err
	} else if isSolid {
		if err := propagateSolidity(transaction.GetHash()); err != nil {
			return false, err
		}
	}

	return false, nil
}

func propagateSolidity(transactionHash ternary.Trinary) errors.IdentifiableError {
	if approvers, err := GetApprovers(transactionHash, NewApprovers); err != nil {
		return err
	} else {
		for _, approverHash := range approvers.GetHashes() {
			if approver, err := GetTransaction(approverHash); err != nil {
				return err
			} else if approver != nil {
				if isSolid, err := checkSolidity(approver); err != nil {
					return err
				} else if isSolid {
					if err := propagateSolidity(approver.GetHash()); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func processRawTransaction(plugin *node.Plugin, rawTransaction *transaction.Transaction) {
	var newTransaction bool
	if tx, err := GetTransaction(rawTransaction.Hash.ToTrinary(), func(transactionHash ternary.Trinary) *Transaction {
		newTransaction = true

		return &Transaction{rawTransaction: rawTransaction}
	}); err != nil {
		plugin.LogFailure(err.Error())
	} else if newTransaction {
		go processTransaction(plugin, tx)
	}
}

func processTransaction(plugin *node.Plugin, transaction *Transaction) {
	transactionHash := transaction.GetHash()

	// register tx as approver for trunk
	if trunkApprovers, err := GetApprovers(transaction.GetTrunkTransactionHash(), NewApprovers); err != nil {
		plugin.LogFailure(err.Error())

		return
	} else {
		trunkApprovers.Add(transactionHash)
	}

	// register tx as approver for branch
	if branchApprovers, err := GetApprovers(transaction.GetBranchTransactionHash(), NewApprovers); err != nil {
		plugin.LogFailure(err.Error())

		return
	} else {
		branchApprovers.Add(transactionHash)
	}

	// update the solidity flags of this transaction and its approvers
	if _, err := IsSolid(transaction); err != nil {
		plugin.LogFailure(err.Error())

		return
	}
}
