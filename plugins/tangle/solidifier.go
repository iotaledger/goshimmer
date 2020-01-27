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

const UnsolidInterval = 30

var (
	workerCount = runtime.NumCPU()
	workerPool  *workerpool.WorkerPool
	unsolidTxs  *UnsolidTxs

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

	unsolidTxs = NewUnsolidTxs()

	gossip.Events.TransactionReceived.Attach(events.NewClosure(func(ev *gossip.TransactionReceivedEvent) {
		metaTx := meta_transaction.FromBytes(ev.Data)
		if err := metaTx.Validate(); err != nil {
			log.Warnf("invalid transaction: %s", err)
			return
		}

		workerPool.Submit(metaTx)
	}))
}

func runSolidifier() {
	log.Info("Starting Solidifier ...")

	daemon.BackgroundWorker("Tangle Solidifier", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Solidifier ... done")
		workerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Solidifier ...")
		workerPool.StopAndWait()
		log.Info("Stopping Solidifier ... done")
	}, shutdown.ShutdownPrioritySolidifier)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// updateSolidity checks the solid flag of a single transaction; if it is not marked as solid,
// recursively request all non solid parent transactions.
func updateSolidity(transaction *value_transaction.ValueTransaction) (bool, error) {
	// abort if transaction is solid already
	txMetadata, metaDataErr := GetTransactionMetadata(transaction.GetHash(), transactionmetadata.New)
	if metaDataErr != nil {
		return false, metaDataErr
	}
	if txMetadata.GetSolid() {
		log.Debugw("transaction is solid", "hash", transaction.GetHash())
		return true, nil
	}

	branchSolid, err := isSolid(transaction.GetBranchTransactionHash())
	if err != nil {
		return false, err
	}
	trunkSolid, err := isSolid(transaction.GetTrunkTransactionHash())
	if err != nil {
		return false, err
	}

	if !branchSolid || !trunkSolid {
		log.Debugw("transaction not solid",
			"hash", transaction.GetHash(),
			"branchSolid", branchSolid,
			"trunkSolid", trunkSolid,
		)
		return false, nil
	}

	if txMetadata.SetSolid(true) {
		log.Debugw("transaction solidified", "hash", transaction.GetHash())
		unsolidTxs.Remove(transaction.GetHash())
		Events.TransactionSolid.Trigger(transaction)
	}
	return true, nil
}

func isSolid(hash trinary.Hash) (bool, error) {
	if hash == meta_transaction.BRANCH_NULL_HASH {
		return true, nil
	}
	transaction, err := GetTransaction(hash)
	if err != nil {
		return false, err
	}
	// if we don't know the transaction, request it
	if transaction == nil {
		if unsolidTxs.Add(hash) {
			requestTransaction(hash)
		}
		return false, nil
	}
	metadata, err := GetTransactionMetadata(transaction.GetHash(), transactionmetadata.New)
	if err != nil {
		return false, err
	}
	// if it is marked solid, we are done
	if metadata.GetSolid() {
		return true, nil
	}
	// if we are not requesting that transaction already
	if !unsolidTxs.Contains(transaction.GetHash()) {
		return updateSolidity(transaction)
	}
	return false, nil
}

// Checks and updates the solid flag of a transaction and its approvers (future cone).
func IsSolid(transaction *value_transaction.ValueTransaction) (bool, error) {
	if isSolid, err := updateSolidity(transaction); err != nil {
		return false, err
	} else if isSolid {
		if err := propagateSolidity(transaction.GetHash()); err != nil {
			return false, err //should we return true?
		}
		return true, nil
	}
	return false, nil
}

func propagateSolidity(transactionHash trinary.Trytes) error {
	if transactionApprovers, err := GetApprovers(transactionHash, approvers.New); err != nil {
		return err
	} else {
		for _, approverHash := range transactionApprovers.GetHashes() {
			if approver, err := GetTransaction(approverHash); err != nil {
				return err
			} else if approver != nil {
				if isSolid, err := updateSolidity(approver); err != nil {
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

func processMetaTransaction(metaTransaction *meta_transaction.MetaTransaction) {
	var newTransaction bool
	tx, err := GetTransaction(metaTransaction.GetHash(), func(transactionHash trinary.Trytes) *value_transaction.ValueTransaction {
		newTransaction = true

		tx := value_transaction.FromMetaTransaction(metaTransaction)
		tx.SetModified(true)
		return tx
	})
	if err != nil {
		log.Errorf("Unable to process transaction %s: %s", metaTransaction.GetHash(), err.Error())
		return
	}
	if newTransaction {
		log.Debugw("process new transaction", "hash", tx.GetHash())
		updateUnsolidTxs(tx)
		processTransaction(tx)
	}
}

func processTransaction(transaction *value_transaction.ValueTransaction) {
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

	// update the solidity flags of this transaction and its approvers
	if _, err := IsSolid(transaction); err != nil {
		log.Errorf("Unable to check solidity: %s", err.Error())
		return
	}
}

func updateUnsolidTxs(tx *value_transaction.ValueTransaction) {
	unsolidTxs.Remove(tx.GetHash())
	targetTime := time.Now().Add(time.Duration(-UnsolidInterval) * time.Second)
	txs := unsolidTxs.Update(targetTime)
	for _, tx := range txs {
		requestTransaction(tx)
	}
}

func requestTransaction(hash trinary.Trytes) {
	if requester == nil {
		return
	}

	log.Debugw("Requesting tx", "hash", hash)
	requester.RequestTransaction(hash)
}
