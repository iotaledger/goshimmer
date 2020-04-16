package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	MAX_MISSING_TIME_BEFORE_CLEANUP = 30 * time.Second
	MISSING_CHECK_INTERVAL          = 5 * time.Second
)

type Tangle struct {
	messageStorage         *objectstorage.ObjectStorage
	messageMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingMessageStorage  *objectstorage.ObjectStorage

	Events Events

	storeMessageWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func messageFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return message.StorableObjectFromKey(key)
}

func approverFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return ApproverFromStorageKey(key)
}

func missingMessageFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingMessageFromStorageKey(key)
}

// Constructor for the tangle.
func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.MessageLayer)

	result = &Tangle{
		messageStorage:         osFactory.New(PrefixMessage, messageFactory, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage: osFactory.New(PrefixMessageMetadata, MessageMetadataFromStorageKey, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:        osFactory.New(PrefixApprovers, approverFactory, objectstorage.CacheTime(10*time.Second), objectstorage.PartitionKey(message.IdLength, message.IdLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, missingMessageFactory, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),

		Events: *newEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)

	return
}

// Attaches a new transaction to the tangle.
func (tangle *Tangle) AttachMessage(transaction *message.Message) {
	tangle.storeMessageWorkerPool.Submit(func() { tangle.storeMessageWorker(transaction) })
}

// Retrieves a transaction from the tangle.
func (tangle *Tangle) Message(transactionId message.Id) *message.CachedMessage {
	return &message.CachedMessage{CachedObject: tangle.messageStorage.Load(transactionId[:])}
}

// Retrieves the metadata of a transaction from the tangle.
func (tangle *Tangle) MessageMetadata(transactionId message.Id) *CachedMessageMetadata {
	return &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Load(transactionId[:])}
}

// Approvers retrieves the approvers of a transaction from the tangle.
func (tangle *Tangle) Approvers(transactionId message.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedApprover{CachedObject: cachedObject})

		return true
	}, transactionId[:])

	return approvers
}

// Deletes a transaction from the tangle (i.e. for local snapshots)
func (tangle *Tangle) DeleteMessage(messageId message.Id) {
	tangle.Message(messageId).Consume(func(currentTransaction *message.Message) {
		trunkTransactionId := currentTransaction.TrunkId()
		tangle.deleteApprover(trunkTransactionId, messageId)

		branchTransactionId := currentTransaction.BranchId()
		if branchTransactionId != trunkTransactionId {
			tangle.deleteApprover(branchTransactionId, messageId)
		}

		tangle.messageMetadataStorage.Delete(messageId[:])
		tangle.messageStorage.Delete(messageId[:])

		tangle.Events.TransactionRemoved.Trigger(messageId)
	})
}

// Marks the tangle as stopped, so it will not accept any new transactions (waits for all backgroundTasks to finish.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storeMessageWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	tangle.messageStorage.Shutdown()
	tangle.messageMetadataStorage.Shutdown()
	tangle.approverStorage.Shutdown()
	tangle.missingMessageStorage.Shutdown()

	return tangle
}

// Resets the database and deletes all objects (good for testing or "node resets").
func (tangle *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.messageStorage,
		tangle.messageMetadataStorage,
		tangle.approverStorage,
		tangle.missingMessageStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

// Worker that stores the transactions and calls the corresponding storage events"
func (tangle *Tangle) storeMessageWorker(tx *message.Message) {
	// store transaction
	var cachedTransaction *message.CachedMessage
	if _tmp, transactionIsNew := tangle.messageStorage.StoreIfAbsent(tx); !transactionIsNew {
		return
	} else {
		cachedTransaction = &message.CachedMessage{CachedObject: _tmp}
	}

	// store transaction metadata
	transactionId := tx.Id()
	cachedTransactionMetadata := &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Store(NewMessageMetadata(transactionId))}

	// store trunk approver
	trunkTransactionID := tx.TrunkId()
	tangle.approverStorage.Store(NewApprover(trunkTransactionID, transactionId)).Release()

	// store branch approver
	if branchTransactionID := tx.BranchId(); branchTransactionID != trunkTransactionID {
		tangle.approverStorage.Store(NewApprover(branchTransactionID, transactionId)).Release()
	}

	// trigger events
	if tangle.missingMessageStorage.DeleteIfPresent(transactionId[:]) {
		tangle.Events.MissingTransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata)
	}
	tangle.Events.TransactionAttached.Trigger(cachedTransaction, cachedTransactionMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyMessageWorker(cachedTransaction, cachedTransactionMetadata)
	})
}

// Worker that solidifies the transactions (recursively from past to present).
func (tangle *Tangle) solidifyMessageWorker(cachedTransaction *message.CachedMessage, cachedTransactionMetadata *CachedMessageMetadata) {
	isTransactionMarkedAsSolid := func(transactionId message.Id) bool {
		if transactionId == message.EmptyId {
			return true
		}

		transactionMetadataCached := tangle.MessageMetadata(transactionId)
		if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
			transactionMetadataCached.Release()

			// if transaction is missing and was not reported as missing, yet
			if cachedMissingTransaction, missingTransactionStored := tangle.missingMessageStorage.StoreIfAbsent(NewMissingMessage(transactionId)); missingTransactionStored {
				cachedMissingTransaction.Consume(func(object objectstorage.StorableObject) {
					tangle.monitorMissingMessageWorker(object.(*MissingMessage).GetTransactionId())
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

	isTransactionSolid := func(transaction *message.Message, transactionMetadata *MessageMetadata) bool {
		if transaction == nil || transaction.IsDeleted() {
			return false
		}

		if transactionMetadata == nil || transactionMetadata.IsDeleted() {
			return false
		}

		if transactionMetadata.IsSolid() {
			return true
		}

		return isTransactionMarkedAsSolid(transaction.TrunkId()) && isTransactionMarkedAsSolid(transaction.BranchId())
	}

	popElementsFromStack := func(stack *list.List) (*message.CachedMessage, *CachedMessageMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedTransaction := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedTransactionMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)

		return currentCachedTransaction.(*message.CachedMessage), currentCachedTransactionMetadata.(*CachedMessageMetadata)
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

			tangle.Approvers(currentTransaction.Id()).Consume(func(approver *Approver) {
				approverTransactionId := approver.ReferencedMessageId()

				solidificationStack.PushBack([2]interface{}{
					tangle.Message(approverTransactionId),
					tangle.MessageMetadata(approverTransactionId),
				})
			})
		}

		// release cached results
		currentCachedTransaction.Release()
		currentCachedTransactionMetadata.Release()
	}
}

// Worker that Monitors the missing transactions (by scheduling regular checks).
func (tangle *Tangle) monitorMissingMessageWorker(transactionId message.Id) {
	var scheduleNextMissingCheck func(transactionId message.Id)
	scheduleNextMissingCheck = func(transactionId message.Id) {
		time.AfterFunc(MISSING_CHECK_INTERVAL, func() {
			tangle.missingMessageStorage.Load(transactionId[:]).Consume(func(object objectstorage.StorableObject) {
				missingTransaction := object.(*MissingMessage)

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

func (tangle *Tangle) deleteApprover(approvedTransaction message.Id, approvingTransaction message.Id) {
	idToDelete := make([]byte, message.IdLength+message.IdLength)
	copy(idToDelete[:message.IdLength], approvedTransaction[:])
	copy(idToDelete[message.IdLength:], approvingTransaction[:])
	tangle.approverStorage.Delete(idToDelete)
}

// Deletes a transaction and all of its approvers (recursively).
func (tangle *Tangle) deleteSubtangle(transactionId message.Id) {
	cleanupStack := list.New()
	cleanupStack.PushBack(transactionId)

	processedTransactions := make(map[message.Id]types.Empty)
	processedTransactions[transactionId] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentTransactionId := currentStackEntry.Value.(message.Id)
		cleanupStack.Remove(currentStackEntry)

		tangle.DeleteMessage(currentTransactionId)

		tangle.Approvers(currentTransactionId).Consume(func(approver *Approver) {
			approverId := approver.ReferencedMessageId()

			if _, transactionProcessed := processedTransactions[approverId]; !transactionProcessed {
				cleanupStack.PushBack(approverId)

				processedTransactions[approverId] = types.Void
			}
		})
	}
}
