package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/approver"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/missingtransaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	MAX_MISSING_TIME_BEFORE_CLEANUP = 30 * time.Second
	MISSING_CHECK_INTERVAL          = 5 * time.Second
)

type Tangle struct {
	storageId []byte

	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	approverStorage            *objectstorage.ObjectStorage
	missingTransactionsStorage *objectstorage.ObjectStorage

	Events Events

	storeTransactionsWorkerPool async.WorkerPool
	solidifierWorkerPool        async.WorkerPool
	cleanupWorkerPool           async.WorkerPool
}

// Constructor for the tangle.
func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId:                  storageId,
		transactionStorage:         objectstorage.New(badgerInstance, append(storageId, storageprefix.TangleTransaction...), transaction.FromStorage, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		transactionMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.TangleTransactionMetadata...), transactionmetadata.FromStorage, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:            objectstorage.New(badgerInstance, append(storageId, storageprefix.TangleApprovers...), approver.FromStorage, objectstorage.CacheTime(10*time.Second), objectstorage.PartitionKey(transaction.IdLength, transaction.IdLength), objectstorage.LeakDetectionEnabled(false)),
		missingTransactionsStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.TangleMissingTransaction...), missingtransaction.FromStorage, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),

		Events: *newEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)

	return
}

// Returns the storage id of this tangle (can be used to create ontologies that follow the storage of the main tangle).
func (tangle *Tangle) GetStorageId() []byte {
	return tangle.storageId
}

// Attaches a new transaction to the tangle.
func (tangle *Tangle) AttachTransaction(transaction *transaction.Transaction) {
	tangle.storeTransactionsWorkerPool.Submit(func() { tangle.storeTransactionWorker(transaction) })
}

// Retrieves a transaction from the tangle.
func (tangle *Tangle) GetTransaction(transactionId transaction.Id) *transaction.CachedTransaction {
	return &transaction.CachedTransaction{CachedObject: tangle.transactionStorage.Load(transactionId[:])}
}

// Retrieves the metadata of a transaction from the tangle.
func (tangle *Tangle) GetTransactionMetadata(transactionId transaction.Id) *transactionmetadata.CachedTransactionMetadata {
	return &transactionmetadata.CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(transactionId[:])}
}

// GetApprovers retrieves the approvers of a transaction from the tangle.
func (tangle *Tangle) GetApprovers(transactionId transaction.Id) approver.CachedApprovers {
	approvers := make(approver.CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &approver.CachedApprover{CachedObject: cachedObject})

		return true
	}, transactionId[:])

	return approvers
}

// Deletes a transaction from the tangle (i.e. for local snapshots)
func (tangle *Tangle) DeleteTransaction(transactionId transaction.Id) {
	tangle.GetTransaction(transactionId).Consume(func(currentTransaction *transaction.Transaction) {
		trunkTransactionId := currentTransaction.GetTrunkTransactionId()
		tangle.deleteApprover(trunkTransactionId, transactionId)

		branchTransactionId := currentTransaction.GetBranchTransactionId()
		if branchTransactionId != trunkTransactionId {
			tangle.deleteApprover(branchTransactionId, transactionId)
		}

		tangle.transactionMetadataStorage.Delete(transactionId[:])
		tangle.transactionStorage.Delete(transactionId[:])

		tangle.Events.TransactionRemoved.Trigger(transactionId)
	})
}

// Marks the tangle as stopped, so it will not accept any new transactions (waits for all backgroundTasks to finish.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storeTransactionsWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	tangle.transactionStorage.Shutdown()
	tangle.transactionMetadataStorage.Shutdown()
	tangle.approverStorage.Shutdown()
	tangle.missingTransactionsStorage.Shutdown()

	return tangle
}

// Resets the database and deletes all objects (good for testing or "node resets").
func (tangle *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.transactionStorage,
		tangle.transactionMetadataStorage,
		tangle.approverStorage,
		tangle.missingTransactionsStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

// Worker that stores the transactions and calls the corresponding storage events"
func (tangle *Tangle) storeTransactionWorker(tx *transaction.Transaction) {
	// store transaction
	var cachedTransaction *transaction.CachedTransaction
	if _tmp, transactionIsNew := tangle.transactionStorage.StoreIfAbsent(tx); !transactionIsNew {
		return
	} else {
		cachedTransaction = &transaction.CachedTransaction{CachedObject: _tmp}
	}

	// store transaction metadata
	transactionId := tx.GetId()
	cachedTransactionMetadata := &transactionmetadata.CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Store(transactionmetadata.New(transactionId))}

	// store trunk approver
	trunkTransactionID := tx.GetTrunkTransactionId()
	tangle.approverStorage.Store(approver.New(trunkTransactionID, transactionId)).Release()

	// store branch approver
	if branchTransactionID := tx.GetBranchTransactionId(); branchTransactionID != trunkTransactionID {
		tangle.approverStorage.Store(approver.New(branchTransactionID, transactionId)).Release()
	}

	// trigger events
	if tangle.missingTransactionsStorage.DeleteIfPresent(transactionId[:]) {
		tangle.Events.MissingTransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata)
	}
	tangle.Events.TransactionAttached.Trigger(cachedTransaction, cachedTransactionMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyTransactionWorker(cachedTransaction, cachedTransactionMetadata)
	})
}

// Worker that solidifies the transactions (recursively from past to present).
func (tangle *Tangle) solidifyTransactionWorker(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *transactionmetadata.CachedTransactionMetadata) {
	isTransactionMarkedAsSolid := func(transactionId transaction.Id) bool {
		if transactionId == transaction.EmptyId {
			return true
		}

		transactionMetadataCached := tangle.GetTransactionMetadata(transactionId)
		if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
			transactionMetadataCached.Release()

			// if transaction is missing and was not reported as missing, yet
			if cachedMissingTransaction, missingTransactionStored := tangle.missingTransactionsStorage.StoreIfAbsent(missingtransaction.New(transactionId)); missingTransactionStored {
				cachedMissingTransaction.Consume(func(object objectstorage.StorableObject) {
					tangle.monitorMissingTransactionWorker(object.(*missingtransaction.MissingTransaction).GetTransactionId())
				})
			}

			return false
		} else if !transactionMetadata.IsSolid() {
			transactionMetadataCached.Release()

			return false
		}
		transactionMetadataCached.Release()

		return true
	}

	isTransactionSolid := func(transaction *transaction.Transaction, transactionMetadata *transactionmetadata.TransactionMetadata) bool {
		if transaction == nil || transaction.IsDeleted() {
			return false
		}

		if transactionMetadata == nil || transactionMetadata.IsDeleted() {
			return false
		}

		if transactionMetadata.IsSolid() {
			return true
		}

		return isTransactionMarkedAsSolid(transaction.GetTrunkTransactionId()) && isTransactionMarkedAsSolid(transaction.GetBranchTransactionId())
	}

	popElementsFromStack := func(stack *list.List) (*transaction.CachedTransaction, *transactionmetadata.CachedTransactionMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedTransaction := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedTransactionMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)

		return currentCachedTransaction.(*transaction.CachedTransaction), currentCachedTransactionMetadata.(*transactionmetadata.CachedTransactionMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedTransaction, cachedTransactionMetadata})

	// process transactions that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedTransaction, currentCachedTransactionMetadata := popElementsFromStack(solidificationStack)

		currentTransaction := currentCachedTransaction.Unwrap()
		currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
		if currentTransaction == nil || currentTransactionMetadata == nil {
			currentCachedTransaction.Release()
			currentCachedTransactionMetadata.Release()

			continue
		}

		// if current transaction is solid and was not marked as solid before: mark as solid and propagate
		if isTransactionSolid(currentTransaction, currentTransactionMetadata) && currentTransactionMetadata.SetSolid(true) {
			tangle.Events.TransactionSolid.Trigger(currentCachedTransaction, currentCachedTransactionMetadata)

			tangle.GetApprovers(currentTransaction.GetId()).Consume(func(approver *approver.Approver) {
				approverTransactionId := approver.GetApprovingTransactionId()

				solidificationStack.PushBack([2]interface{}{
					tangle.GetTransaction(approverTransactionId),
					tangle.GetTransactionMetadata(approverTransactionId),
				})
			})
		}

		// release cached results
		currentCachedTransaction.Release()
		currentCachedTransactionMetadata.Release()
	}
}

// Worker that Monitors the missing transactions (by scheduling regular checks).
func (tangle *Tangle) monitorMissingTransactionWorker(transactionId transaction.Id) {
	var scheduleNextMissingCheck func(transactionId transaction.Id)
	scheduleNextMissingCheck = func(transactionId transaction.Id) {
		time.AfterFunc(MISSING_CHECK_INTERVAL, func() {
			tangle.missingTransactionsStorage.Load(transactionId[:]).Consume(func(object objectstorage.StorableObject) {
				missingTransaction := object.(*missingtransaction.MissingTransaction)

				if time.Since(missingTransaction.GetMissingSince()) >= MAX_MISSING_TIME_BEFORE_CLEANUP {
					tangle.cleanupWorkerPool.Submit(func() {
						tangle.Events.TransactionUnsolidifiable.Trigger(transactionId)

						tangle.deleteSubtangle(missingTransaction.GetTransactionId())
					})
				} else {
					// TRIGGER STILL MISSING EVENT?

					scheduleNextMissingCheck(transactionId)
				}
			})
		})
	}

	tangle.Events.TransactionMissing.Trigger(transactionId)

	scheduleNextMissingCheck(transactionId)
}

func (tangle *Tangle) deleteApprover(approvedTransaction transaction.Id, approvingTransaction transaction.Id) {
	idToDelete := make([]byte, transaction.IdLength+transaction.IdLength)
	copy(idToDelete[:transaction.IdLength], approvedTransaction[:])
	copy(idToDelete[transaction.IdLength:], approvingTransaction[:])
	tangle.approverStorage.Delete(idToDelete)
}

// Deletes a transaction and all of its approvers (recursively).
func (tangle *Tangle) deleteSubtangle(transactionId transaction.Id) {
	cleanupStack := list.New()
	cleanupStack.PushBack(transactionId)

	processedTransactions := make(map[transaction.Id]types.Empty)
	processedTransactions[transactionId] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentTransactionId := currentStackEntry.Value.(transaction.Id)
		cleanupStack.Remove(currentStackEntry)

		tangle.DeleteTransaction(currentTransactionId)

		tangle.GetApprovers(currentTransactionId).Consume(func(approver *approver.Approver) {
			approverId := approver.GetApprovingTransactionId()

			if _, transactionProcessed := processedTransactions[approverId]; !transactionProcessed {
				cleanupStack.PushBack(approverId)

				processedTransactions[approverId] = types.Void
			}
		})
	}
}
