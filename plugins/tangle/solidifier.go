package tangle

import (
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/errors"
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
			//log.Info("Missing Branch (nil)", transaction.GetValue(), "Missing ", transaction.GetBranchTransactionHash())

			// add BranchTransactionHash to the unsolid txs and send a transaction request
			unsolidTxs.Add(transaction.GetBranchTransactionHash())
			requestTransaction(transaction.GetBranchTransactionHash())

			return
		} else if branchTransactionMetadata, branchErr := GetTransactionMetadata(branchTransaction.GetHash(), transactionmetadata.New); branchErr != nil {
			err = branchErr
			//log.Info("Missing Branch", transaction.GetValue())
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
			//log.Info("Missing Trunk (nil)", transaction.GetValue())

			// add TrunkTransactionHash to the unsolid txs and send a transaction request
			unsolidTxs.Add(transaction.GetTrunkTransactionHash())
			requestTransaction(transaction.GetTrunkTransactionHash())

			return
		} else if trunkTransactionMetadata, trunkErr := GetTransactionMetadata(trunkTransaction.GetHash(), transactionmetadata.New); trunkErr != nil {
			err = trunkErr
			//log.Info("Missing Trunk", transaction.GetValue())
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
		//log.Info("Solid ", transaction.GetValue())
		if err := propagateSolidity(transaction.GetHash()); err != nil {
			return false, err //should we return true?
		}
		return true, nil
	}
	//log.Info("Not solid ", transaction.GetValue())
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

func processMetaTransaction(metaTransaction *meta_transaction.MetaTransaction) {
	var newTransaction bool
	if tx, err := GetTransaction(metaTransaction.GetHash(), func(transactionHash trinary.Trytes) *value_transaction.ValueTransaction {
		newTransaction = true

		tx := value_transaction.FromMetaTransaction(metaTransaction)
		tx.SetModified(true)
		return tx
	}); err != nil {
		log.Errorf("Unable to load transaction %s: %s", metaTransaction.GetHash(), err.Error())
	} else if newTransaction {
		updateUnsolidTxs(tx)
		processTransaction(tx)
	}
}

func processTransaction(transaction *value_transaction.ValueTransaction) {
	Events.TransactionStored.Trigger(transaction)

	// store transaction hash for address in DB
	err := StoreTransactionHashForAddressInDatabase(
		&TxHashForAddress{
			Address: transaction.GetAddress(),
			TxHash:  transaction.GetHash(),
		},
	)
	if err != nil {
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
	_, err = IsSolid(transaction)
	if err != nil {
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
