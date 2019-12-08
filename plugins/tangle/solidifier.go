package tangle

import (
	"runtime"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/model/approvers"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/workerpool"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/trinary"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var workerPool *workerpool.WorkerPool

func configureSolidifier(plugin *node.Plugin) {
	workerPool = workerpool.New(func(task workerpool.Task) {
		processMetaTransaction(plugin, task.Param(0).(*meta_transaction.MetaTransaction))

		task.Return(nil)
	}, workerpool.WorkerCount(WORKER_COUNT), workerpool.QueueSize(10000))

	gossip.Events.NewTransaction.Attach(events.NewClosure(func(ev *gossip.NewTransactionEvent) {
		//log.Info("New Transaction", ev.Body)
		pTx := &pb.Transaction{}
		proto.Unmarshal(ev.Body, pTx)
		metaTx := meta_transaction.FromBytes(pTx.GetBody())
		workerPool.Submit(metaTx)
	}))

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		log.Info("Stopping Solidifier ...")
		workerPool.Stop()
	}))
}

func runSolidifier(plugin *node.Plugin) {
	log.Info("Starting Solidifier ...")

	daemon.BackgroundWorker("Tangle Solidifier", func() {
		log.Info("Starting Solidifier ... done")
		workerPool.Run()
		log.Info("Stopping Solidifier ... done")
	})
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

	// check solidity of trunk transaction if it is not genesis
	if trunkTransactionHash := transaction.GetTrunkTransactionHash(); trunkTransactionHash != meta_transaction.BRANCH_NULL_HASH {
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

func propagateSolidity(transactionHash trinary.Trytes) errors.IdentifiableError {
	if transactionApprovers, err := GetApprovers(transactionHash, approvers.New); err != nil {
		return err
	} else {
		for _, approverHash := range transactionApprovers.GetHashes() {
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
	if tx, err := GetTransaction(metaTransaction.GetHash(), func(transactionHash trinary.Trytes) *value_transaction.ValueTransaction {
		newTransaction = true

		return value_transaction.FromMetaTransaction(metaTransaction)
	}); err != nil {
		log.Errorf("Unable to load transaction %s: %s", metaTransaction.GetHash(), err.Error())
	} else if newTransaction {
		processTransaction(plugin, tx)
	}
}

func processTransaction(plugin *node.Plugin, transaction *value_transaction.ValueTransaction) {
	Events.TransactionStored.Trigger(transaction)

	transactionHash := transaction.GetHash()

	// register tx as approver for trunk
	if trunkApprovers, err := GetApprovers(transaction.GetTrunkTransactionHash(), approvers.New); err != nil {
		log.Errorf("Unable to get approvers of transaction %s: %s", transaction.GetTrunkTransactionHash(), err.Error())
		return
	} else {
		trunkApprovers.Add(transactionHash)
	}

	// register tx as approver for branch
	if branchApprovers, err := GetApprovers(transaction.GetBranchTransactionHash(), approvers.New); err != nil {
		log.Errorf("Unable to get approvers of transaction %s: %s", transaction.GetBranchTransactionHash(), err.Error())
		return
	} else {
		branchApprovers.Add(transactionHash)
	}

	// update the solidity flags of this transaction and its approvers
	if _, err := IsSolid(transaction); err != nil {
		log.Errorf("Unable to check solidity: %s", err.Error())
		return
	}
}

var WORKER_COUNT = runtime.NumCPU()
