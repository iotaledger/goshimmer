package tangle

import (
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/approvers"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/iotaledger/iota.go/trinary"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

const UnsolidInterval = time.Minute

var (
	workerCount  = runtime.NumCPU()
	workerPool   *workerpool.WorkerPool
	requestedTxs *UnsolidTxs

	requester Requester
)

func SetRequester(req Requester) {
	requester = req
}

func configureSolidifier() {
	workerPool = workerpool.New(func(task workerpool.Task) {
		processMetaTransaction(task.Param(0).(*meta_transaction.MetaTransaction))

		task.Return(nil)
	}, workerpool.WorkerCount(workerCount), workerpool.QueueSize(10000))

	requestedTxs = NewUnsolidTxs()

	gossip.Events.TransactionReceived.Attach(events.NewClosure(func(ev *gossip.TransactionReceivedEvent) {
		metaTx, err := meta_transaction.FromBytes(ev.Data)
		if err != nil {
			log.Warnf("invalid transaction: %s", err)
			return
		}
		if err = metaTx.Validate(); err != nil {
			log.Warnf("invalid transaction: %s", err)
			return
		}
		workerPool.Submit(metaTx)
	}))
}

func runSolidifier() {
	daemon.BackgroundWorker("Tangle Solidifier", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Solidifier ...")
		workerPool.Start()
		log.Info("Starting Solidifier ... done")

		<-shutdownSignal

		log.Info("Stopping Solidifier ...")
		workerPool.StopAndWait()
		log.Info("Stopping Solidifier ... done")
	}, shutdown.ShutdownPrioritySolidifier)

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func processMetaTransaction(metaTransaction *meta_transaction.MetaTransaction) {
	metaTransactionHash := metaTransaction.GetHash()

	var newTransaction bool
	tx, err := GetTransaction(metaTransactionHash, func(transactionHash trinary.Trytes) *value_transaction.ValueTransaction {
		newTransaction = true

		tx := value_transaction.FromMetaTransaction(metaTransaction)
		tx.SetModified(true)
		return tx
	})
	if err != nil {
		log.Errorf("Unable to process transaction %s: %s", metaTransactionHash, err.Error())
		return
	}
	if newTransaction {
		log.Debugw("process new transaction", "hash", tx.GetHash())
		processNewTransaction(tx)
		requestedTxs.Remove(tx.GetHash())
		updateRequestedTxs()
	}
}

func processNewTransaction(transaction *value_transaction.ValueTransaction) {
	Events.TransactionStored.Trigger(transaction)

	// store transaction hash for address in DB
	if err := StoreTransactionHashForAddressInDatabase(
		&TxHashForAddress{
			Address: transaction.GetAddress(),
			TxHash:  transaction.GetHash(),
		},
	); err != nil {
		log.Errorw(err.Error())
	}

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

	isSolid, err := isSolid(transactionHash)
	if err != nil {
		log.Errorf("Unable to check solidity: %s", err.Error())
	}
	// if the transaction was solidified propagate this information
	if isSolid {
		if err := propagateSolidity(transaction.GetHash()); err != nil {
			log.Errorf("Unable to propagate solidity: %s", err.Error())
		}
	}
}

// isSolid checks whether the transaction with the given hash is solid. A transaction is solid, if it is
// either marked as solid or all its referenced transactions are in the database.
func isSolid(hash trinary.Hash) (bool, error) {
	// the genesis is always solid
	if hash == meta_transaction.BRANCH_NULL_HASH {
		return true, nil
	}
	// if the transaction is not in the DB, request it
	transaction, err := GetTransaction(hash)
	if err != nil {
		return false, err
	}
	if transaction == nil {
		if requestedTxs.Add(hash) {
			requestTransaction(hash)
		}
		return false, nil
	}

	// check whether the transaction is marked solid
	metadata, err := GetTransactionMetadata(hash, transactionmetadata.New)
	if err != nil {
		return false, err
	}
	if metadata.GetSolid() {
		return true, nil
	}

	branch := contains(transaction.GetBranchTransactionHash())
	trunk := contains(transaction.GetTrunkTransactionHash())

	if !branch || !trunk {
		return false, nil
	}
	// everything is good, mark the transaction as solid
	return true, markSolid(transaction)
}

func contains(hash trinary.Hash) bool {
	if hash == meta_transaction.BRANCH_NULL_HASH {
		return true
	}
	if contains, _ := ContainsTransaction(hash); !contains {
		if requestedTxs.Add(hash) {
			requestTransaction(hash)
		}
		return false
	}
	return true
}

func isMarkedSolid(hash trinary.Hash) (bool, error) {
	if hash == meta_transaction.BRANCH_NULL_HASH {
		return true, nil
	}
	metadata, err := GetTransactionMetadata(hash, transactionmetadata.New)
	if err != nil {
		return false, err
	}
	return metadata.GetSolid(), nil
}

func markSolid(transaction *value_transaction.ValueTransaction) error {
	txMetadata, err := GetTransactionMetadata(transaction.GetHash(), transactionmetadata.New)
	if err != nil {
		return err
	}
	if txMetadata.SetSolid(true) {
		log.Debugw("transaction solidified", "hash", transaction.GetHash())
		Events.TransactionSolid.Trigger(transaction)
		return propagateSolidity(transaction.GetHash())
	}
	return nil
}

func propagateSolidity(transactionHash trinary.Trytes) error {
	approvingTransactions, err := GetApprovers(transactionHash, approvers.New)
	if err != nil {
		return err
	}
	for _, hash := range approvingTransactions.GetHashes() {
		approver, err := GetTransaction(hash)
		if err != nil {
			return err
		}
		if approver != nil {
			branchSolid, err := isMarkedSolid(approver.GetBranchTransactionHash())
			if err != nil {
				return err
			}
			if !branchSolid {
				continue
			}
			trunkSolid, err := isMarkedSolid(approver.GetTrunkTransactionHash())
			if err != nil {
				return err
			}
			if !trunkSolid {
				continue
			}

			if err := markSolid(approver); err != nil {
				return err
			}
		}
	}
	return nil
}

func updateRequestedTxs() {
	targetTime := time.Now().Add(-UnsolidInterval)
	txs := requestedTxs.Update(targetTime)
	for _, txHash := range txs {
		if contains, _ := ContainsTransaction(txHash); contains {
			requestedTxs.Remove(txHash)
			continue
		}
		requestTransaction(txHash)
	}
}

func requestTransaction(hash trinary.Trytes) {
	if requester == nil {
		return
	}

	log.Debugw("Requesting tx", "hash", hash)
	requester.RequestTransaction(hash)
}
