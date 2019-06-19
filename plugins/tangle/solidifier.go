package tangle

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/approvers"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

//var solidifierChan = make(chan *value_transaction.ValueTransaction, 1000)

const NUMBER_OF_WORKERS = 300

var tasksChan = make(chan *meta_transaction.MetaTransaction, NUMBER_OF_WORKERS)

func configureSolidifier(plugin *node.Plugin) {
	for i := 0; i < NUMBER_OF_WORKERS; i++ {
		go func() {
			for {
				select {
				case <-daemon.ShutdownSignal:
					return
				case rawTransaction := <-tasksChan:
					processMetaTransaction(plugin, rawTransaction)
				}
			}
		}()
	}

	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(rawTransaction *meta_transaction.MetaTransaction) {
		tasksChan <- rawTransaction
	}))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// Checks and updates the solid flag of a single transaction.
func checkSolidity(transaction *value_transaction.ValueTransaction) (result bool, err errors.IdentifiableError) {
	// abort if transaction is solid already
	txMetadata, metaDataErr := GetTransactionMetadata(transaction.GetHash(), transactionmetadata.New)
	if metaDataErr != nil {
		err = metaDataErr

		return
	} else if txMetadata.GetSolid() {
		result = true

		return
	}

	// check solidity of branch transaction if it is not genesis
	if branchTransactionHash := transaction.GetBranchTransactionHash(); branchTransactionHash != meta_transaction.BRANCH_NULL_HASH {
		// abort if branch transaction is missing
		if branchTransaction, branchErr := GetTransaction(branchTransactionHash); branchErr != nil {
			err = branchErr

			return
		} else if branchTransaction == nil {
			return
		} else if branchTransactionMetadata, branchErr := GetTransactionMetadata(branchTransaction.GetHash(), transactionmetadata.New); branchErr != nil {
			err = branchErr

			return
		} else if !branchTransactionMetadata.GetSolid() {
			return
		}
	}

	// check solidity of branch transaction if it is not genesis
	if trunkTransactionHash := transaction.GetBranchTransactionHash(); trunkTransactionHash != meta_transaction.BRANCH_NULL_HASH {
		if trunkTransaction, trunkErr := GetTransaction(trunkTransactionHash); trunkErr != nil {
			err = trunkErr

			return
		} else if trunkTransaction == nil {
			return
		} else if trunkTransactionMetadata, trunkErr := GetTransactionMetadata(trunkTransaction.GetHash(), transactionmetadata.New); trunkErr != nil {
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
func IsSolid(transaction *value_transaction.ValueTransaction) (bool, errors.IdentifiableError) {
	if isSolid, err := checkSolidity(transaction); err != nil {
		return false, err
	} else if isSolid {
		if err := propagateSolidity(transaction.GetHash()); err != nil {
			return false, err
		}
	}

	return false, nil
}

func propagateSolidity(transactionHash ternary.Trytes) errors.IdentifiableError {
	if approvers, err := GetApprovers(transactionHash, approvers.New); err != nil {
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

func processMetaTransaction(plugin *node.Plugin, metaTransaction *meta_transaction.MetaTransaction) {
	var newTransaction bool
	if tx, err := GetTransaction(metaTransaction.GetHash(), func(transactionHash ternary.Trytes) *value_transaction.ValueTransaction {
		newTransaction = true

		return value_transaction.FromMetaTransaction(metaTransaction)
	}); err != nil {
		plugin.LogFailure(err.Error())
	} else if newTransaction {
		processTransaction(plugin, tx)
	}
}

func processTransaction(plugin *node.Plugin, transaction *value_transaction.ValueTransaction) {
	Events.TransactionStored.Trigger(transaction)

	transactionHash := transaction.GetHash()

	// register tx as approver for trunk
	if trunkApprovers, err := GetApprovers(transaction.GetTrunkTransactionHash(), approvers.New); err != nil {
		plugin.LogFailure(err.Error())

		return
	} else {
		trunkApprovers.Add(transactionHash)
	}

	// register tx as approver for branch
	if branchApprovers, err := GetApprovers(transaction.GetBranchTransactionHash(), approvers.New); err != nil {
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
	//solidifierChan <- transaction
}
