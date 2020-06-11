package tangle

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	branchManager *branchmanager.BranchManager

	payloadStorage             *objectstorage.ObjectStorage
	payloadMetadataStorage     *objectstorage.ObjectStorage
	approverStorage            *objectstorage.ObjectStorage
	missingPayloadStorage      *objectstorage.ObjectStorage
	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	attachmentStorage          *objectstorage.ObjectStorage
	outputStorage              *objectstorage.ObjectStorage
	consumerStorage            *objectstorage.ObjectStorage

	Events Events

	workerPool        async.WorkerPool
	cleanupWorkerPool async.WorkerPool
}

// New is the constructor of a Tangle and creates a new Tangle object from the given details.
func New(store kvstore.KVStore) (tangle *Tangle) {
	osFactory := objectstorage.NewFactory(store, storageprefix.ValueTransfers)

	tangle = &Tangle{
		branchManager: branchmanager.New(store),

		payloadStorage:             osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage:     osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:      osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:            osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IDLength, payload.IDLength), objectstorage.KeysOnly(true)),
		transactionStorage:         osFactory.New(osTransaction, osTransactionFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		transactionMetadataStorage: osFactory.New(osTransactionMetadata, osTransactionMetadataFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		attachmentStorage:          osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(transaction.IDLength, payload.IDLength), osLeakDetectionOption),
		outputStorage:              osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		consumerStorage:            osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second), osLeakDetectionOption),

		Events: *newEvents(),
	}
	tangle.setupDAGSynchronization()

	return
}

// region MAIN PUBLIC API //////////////////////////////////////////////////////////////////////////////////////////////

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.workerPool.Submit(func() { tangle.AttachPayloadSync(payload) })
}

// AttachPayloadSync is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) AttachPayloadSync(payloadToStore *payload.Payload) {
	// store the payload models or abort if we have seen the payload already
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		return
	}
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()

	// store transaction models or abort if we have seen this attachment already  (nil == was not stored)
	cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := tangle.storeTransactionModels(payloadToStore)
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()
	if cachedAttachment == nil {
		return
	}
	defer cachedAttachment.Release()

	// store the references between the different entities (we do this after the actual entities were stored, so that
	// all the metadata models exist in the database as soon as the entities are reachable by walks).
	tangle.storePayloadReferences(payloadToStore)

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadToStore.ID().Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedPayloadMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)
	if transactionIsNew {
		tangle.Events.TransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
	}

	// check solidity
	tangle.solidifyPayload(cachedPayload.Retain(), cachedPayloadMetadata.Retain(), cachedTransaction.Retain(), cachedTransactionMetadata.Retain())
}

// SetTransactionPreferred modifies the preferred flag of a transaction. It updates the transactions metadata,
// propagates the changes to the branch DAG and triggers an update of the liked flags in the value tangle.
func (tangle *Tangle) SetTransactionPreferred(transactionID transaction.ID, preferred bool) (modified bool, err error) {
	return tangle.setTransactionPreferred(transactionID, preferred, EventSourceTangle)
}

// SetTransactionFinalized modifies the finalized flag of a transaction. It updates the transactions metadata and
// propagates the changes to the BranchManager if the flag was updated.
func (tangle *Tangle) SetTransactionFinalized(transactionID transaction.ID) (modified bool, err error) {
	return tangle.setTransactionFinalized(transactionID, EventSourceTangle)
}

// ValuePayloadsLiked is checking if the Payloads referenced by the passed in IDs are all liked.
func (tangle *Tangle) ValuePayloadsLiked(payloadIDs ...payload.ID) (liked bool) {
	for _, payloadID := range payloadIDs {
		if payloadID == payload.GenesisID {
			continue
		}

		payloadMetadataFound := tangle.PayloadMetadata(payloadID).Consume(func(payloadMetadata *PayloadMetadata) {
			liked = payloadMetadata.Liked()
		})

		if !payloadMetadataFound || !liked {
			return false
		}
	}

	return true
}

// ValuePayloadsConfirmed is checking if the Payloads referenced by the passed in IDs are all confirmed.
func (tangle *Tangle) ValuePayloadsConfirmed(payloadIDs ...payload.ID) (confirmed bool) {
	for _, payloadID := range payloadIDs {
		if payloadID == payload.GenesisID {
			continue
		}

		payloadMetadataFound := tangle.PayloadMetadata(payloadID).Consume(func(payloadMetadata *PayloadMetadata) {
			confirmed = payloadMetadata.Confirmed()
		})

		if !payloadMetadataFound || !confirmed {
			return false
		}
	}

	return true
}

// BranchManager is the getter for the manager that takes care of creating and updating branches.
func (tangle *Tangle) BranchManager() *branchmanager.BranchManager {
	return tangle.branchManager
}

// LoadSnapshot creates a set of outputs in the value tangle, that are forming the genesis for future transactions.
func (tangle *Tangle) LoadSnapshot(snapshot map[transaction.ID]map[address.Address][]*balance.Balance) {
	for transactionID, addressBalances := range snapshot {
		for outputAddress, balances := range addressBalances {
			input := NewOutput(outputAddress, transactionID, branchmanager.MasterBranchID, balances)
			input.setSolid(true)
			input.SetBranchID(branchmanager.MasterBranchID)

			// store output and abort if the snapshot has already been loaded earlier (output exists in the database)
			cachedOutput, stored := tangle.outputStorage.StoreIfAbsent(input)
			if !stored {
				return
			}

			cachedOutput.Release()
		}
	}
}

// Fork creates a new branch from an existing transaction.
func (tangle *Tangle) Fork(transactionID transaction.ID, conflictingInputs []transaction.OutputID) (forked bool, finalized bool, err error) {
	cachedTransaction := tangle.Transaction(transactionID)
	cachedTransactionMetadata := tangle.TransactionMetadata(transactionID)
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	tx := cachedTransaction.Unwrap()
	if tx == nil {
		err = fmt.Errorf("failed to load transaction '%s'", transactionID)

		return
	}
	txMetadata := cachedTransactionMetadata.Unwrap()
	if txMetadata == nil {
		err = fmt.Errorf("failed to load metadata of transaction '%s'", transactionID)

		return
	}

	// abort if this transaction was finalized already
	if txMetadata.Finalized() {
		finalized = true

		return
	}

	// update / create new branch
	newBranchID := branchmanager.NewBranchID(tx.ID())
	cachedTargetBranch, newBranchCreated := tangle.branchManager.Fork(newBranchID, []branchmanager.BranchID{txMetadata.BranchID()}, conflictingInputs)
	defer cachedTargetBranch.Release()

	// set branch to be preferred if the underlying transaction was marked as preferred
	if txMetadata.Preferred() {
		if _, err = tangle.branchManager.SetBranchPreferred(newBranchID, true); err != nil {
			return
		}
	}

	// abort if the branch existed already
	if !newBranchCreated {
		return
	}

	// move transactions to new branch
	if err = tangle.moveTransactionToBranch(cachedTransaction.Retain(), cachedTransactionMetadata.Retain(), cachedTargetBranch.Retain()); err != nil {
		return
	}

	// trigger events + set result
	tangle.Events.Fork.Trigger(cachedTransaction, cachedTransactionMetadata)
	forked = true

	return
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (tangle *Tangle) Prune() (err error) {
	if err = tangle.branchManager.Prune(); err != nil {
		return
	}

	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.missingPayloadStorage,
		tangle.approverStorage,
		tangle.transactionStorage,
		tangle.transactionMetadataStorage,
		tangle.attachmentStorage,
		tangle.outputStorage,
		tangle.consumerStorage,
	} {
		if err = storage.Prune(); err != nil {
			return
		}
	}

	return
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.workerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.missingPayloadStorage,
		tangle.approverStorage,
		tangle.transactionStorage,
		tangle.transactionMetadataStorage,
		tangle.attachmentStorage,
		tangle.outputStorage,
		tangle.consumerStorage,
	} {
		storage.Shutdown()
	}

	return tangle
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region GETTERS/ITERATORS FOR THE STORED MODELS //////////////////////////////////////////////////////////////////////

// Transaction loads the given transaction from the objectstorage.
func (tangle *Tangle) Transaction(transactionID transaction.ID) *transaction.CachedTransaction {
	return &transaction.CachedTransaction{CachedObject: tangle.transactionStorage.Load(transactionID.Bytes())}
}

// TransactionMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) TransactionMetadata(transactionID transaction.ID) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(transactionID.Bytes())}
}

// TransactionOutput loads the given output from the objectstorage.
func (tangle *Tangle) TransactionOutput(outputID transaction.OutputID) *CachedOutput {
	return &CachedOutput{CachedObject: tangle.outputStorage.Load(outputID.Bytes())}
}

// OutputsOnAddress retrieves all the Outputs that are associated with an address.
func (tangle *Tangle) OutputsOnAddress(address address.Address) (result CachedOutputs) {
	result = make(CachedOutputs)
	tangle.outputStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		outputID, _, err := transaction.OutputIDFromBytes(key)
		if err != nil {
			panic(err)
		}

		result[outputID] = &CachedOutput{CachedObject: cachedObject}

		return true
	}, address.Bytes())

	return
}

// Consumers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) Consumers(outputID transaction.OutputID) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	tangle.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputID.Bytes())

	return consumers
}

// Attachments retrieves the attachment of a payload from the object storage.
func (tangle *Tangle) Attachments(transactionID transaction.ID) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	tangle.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionID.Bytes())

	return attachments
}

// Payload retrieves a payload from the object storage.
func (tangle *Tangle) Payload(payloadID payload.ID) *payload.CachedPayload {
	return &payload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadID.Bytes())}
}

// PayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) PayloadMetadata(payloadID payload.ID) *CachedPayloadMetadata {
	return &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadID.Bytes())}
}

// Approvers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) Approvers(payloadID payload.ID) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, payloadID.Bytes())

	return approvers
}

// ForeachApprovers iterates through the approvers of a payload and calls the passed in consumer function.
func (tangle *Tangle) ForeachApprovers(payloadID payload.ID, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata)) {
	tangle.Approvers(payloadID).Consume(func(approver *PayloadApprover) {
		approvingCachedPayload := tangle.Payload(approver.ApprovingPayloadID())

		approvingCachedPayload.Consume(func(payload *payload.Payload) {
			consume(approvingCachedPayload.Retain(), tangle.PayloadMetadata(approver.ApprovingPayloadID()), tangle.Transaction(payload.Transaction().ID()), tangle.TransactionMetadata(payload.Transaction().ID()))
		})
	})
}

// ForEachConsumers iterates through the transactions that are consuming outputs of the given transactions
func (tangle *Tangle) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata)) {
	seenTransactions := make(map[transaction.ID]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		tangle.Consumers(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(consumer *Consumer) {
			if _, transactionSeen := seenTransactions[consumer.TransactionID()]; !transactionSeen {
				seenTransactions[consumer.TransactionID()] = types.Void

				cachedTransaction := tangle.Transaction(consumer.TransactionID())
				defer cachedTransaction.Release()

				cachedTransactionMetadata := tangle.TransactionMetadata(consumer.TransactionID())
				defer cachedTransactionMetadata.Release()

				tangle.Attachments(consumer.TransactionID()).Consume(func(attachment *Attachment) {
					consume(tangle.Payload(attachment.PayloadID()), tangle.PayloadMetadata(attachment.PayloadID()), cachedTransaction.Retain(), cachedTransactionMetadata.Retain())
				})
			}
		})

		return true
	})
}

// ForEachConsumersAndApprovers calls the passed in consumer for all payloads that either approve the given payload or
// that attach a transaction that spends outputs from the transaction inside the given payload.
func (tangle *Tangle) ForEachConsumersAndApprovers(currentPayload *payload.Payload, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata)) {
	tangle.ForEachConsumers(currentPayload.Transaction(), consume)
	tangle.ForeachApprovers(currentPayload.ID(), consume)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DAG SYNCHRONIZATION //////////////////////////////////////////////////////////////////////////////////////////

// setupDAGSynchronization sets up the behavior how the branch dag and the value tangle and UTXO dag are connected.
func (tangle *Tangle) setupDAGSynchronization() {
	tangle.branchManager.Events.BranchPreferred.Attach(events.NewClosure(tangle.onBranchPreferred))
	tangle.branchManager.Events.BranchUnpreferred.Attach(events.NewClosure(tangle.onBranchUnpreferred))
	tangle.branchManager.Events.BranchLiked.Attach(events.NewClosure(tangle.onBranchLiked))
	tangle.branchManager.Events.BranchDisliked.Attach(events.NewClosure(tangle.onBranchDisliked))
	tangle.branchManager.Events.BranchFinalized.Attach(events.NewClosure(tangle.onBranchFinalized))
	tangle.branchManager.Events.BranchConfirmed.Attach(events.NewClosure(tangle.onBranchConfirmed))
	tangle.branchManager.Events.BranchRejected.Attach(events.NewClosure(tangle.onBranchRejected))
}

// onBranchPreferred gets triggered when a branch in the branch DAG is marked as preferred.
func (tangle *Tangle) onBranchPreferred(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchPreferredChangesToTangle(cachedBranch, true)
}

// onBranchUnpreferred gets triggered when a branch in the branch DAG is marked as NOT preferred.
func (tangle *Tangle) onBranchUnpreferred(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchPreferredChangesToTangle(cachedBranch, false)
}

// onBranchLiked gets triggered when a branch in the branch DAG is marked as liked.
func (tangle *Tangle) onBranchLiked(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchedLikedChangesToTangle(cachedBranch, true)
}

// onBranchDisliked gets triggered when a branch in the branch DAG is marked as disliked.
func (tangle *Tangle) onBranchDisliked(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchedLikedChangesToTangle(cachedBranch, false)
}

// onBranchFinalized gets triggered when a branch in the branch DAG is marked as finalized.
func (tangle *Tangle) onBranchFinalized(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchFinalizedChangesToTangle(cachedBranch)
}

// onBranchConfirmed gets triggered when a branch in the branch DAG is marked as confirmed.
func (tangle *Tangle) onBranchConfirmed(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchConfirmedRejectedChangesToTangle(cachedBranch)
}

// onBranchRejected gets triggered when a branch in the branch DAG is marked as rejected.
func (tangle *Tangle) onBranchRejected(cachedBranch *branchmanager.CachedBranch) {
	tangle.propagateBranchConfirmedRejectedChangesToTangle(cachedBranch)
}

// propagateBranchPreferredChangesToTangle triggers the propagation of preferred status changes of a branch to the value
// tangle and its UTXO DAG.
func (tangle *Tangle) propagateBranchPreferredChangesToTangle(cachedBranch *branchmanager.CachedBranch, preferred bool) {
	cachedBranch.Consume(func(branch *branchmanager.Branch) {
		if !branch.IsAggregated() {
			transactionID, _, err := transaction.IDFromBytes(branch.ID().Bytes())
			if err != nil {
				panic(err) // this should never ever happen
			}

			_, err = tangle.setTransactionPreferred(transactionID, preferred, EventSourceBranchManager)
			if err != nil {
				tangle.Events.Error.Trigger(err)

				return
			}
		}
	})
}

// propagateBranchFinalizedChangesToTangle triggers the propagation of finalized status changes of a branch to the value
// tangle and its UTXO DAG.
func (tangle *Tangle) propagateBranchFinalizedChangesToTangle(cachedBranch *branchmanager.CachedBranch) {
	cachedBranch.Consume(func(branch *branchmanager.Branch) {
		if !branch.IsAggregated() {
			transactionID, _, err := transaction.IDFromBytes(branch.ID().Bytes())
			if err != nil {
				panic(err) // this should never ever happen
			}

			_, err = tangle.setTransactionFinalized(transactionID, EventSourceBranchManager)
			if err != nil {
				tangle.Events.Error.Trigger(err)

				return
			}
		}
	})
}

// propagateBranchedLikedChangesToTangle triggers the propagation of liked status changes of a branch to the value
// tangle and its UTXO DAG.
func (tangle *Tangle) propagateBranchedLikedChangesToTangle(cachedBranch *branchmanager.CachedBranch, liked bool) {
	cachedBranch.Consume(func(branch *branchmanager.Branch) {
		if !branch.IsAggregated() {
			transactionID, _, err := transaction.IDFromBytes(branch.ID().Bytes())
			if err != nil {
				panic(err) // this should never ever happen
			}

			// propagate changes to future cone of transaction (value tangle)
			tangle.propagateValuePayloadLikeUpdates(transactionID, liked)
		}
	})
}

// propagateBranchConfirmedRejectedChangesToTangle triggers the propagation of confirmed and rejected status changes of
// a branch to the value tangle and its UTXO DAG.
func (tangle *Tangle) propagateBranchConfirmedRejectedChangesToTangle(cachedBranch *branchmanager.CachedBranch) {
	cachedBranch.Consume(func(branch *branchmanager.Branch) {
		if !branch.IsAggregated() {
			transactionID, _, err := transaction.IDFromBytes(branch.ID().Bytes())
			if err != nil {
				panic(err) // this should never ever happen
			}

			// propagate changes to future cone of transaction (value tangle)
			tangle.propagateValuePayloadConfirmedRejectedUpdates(transactionID)
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PRIVATE UTILITY METHODS //////////////////////////////////////////////////////////////////////////////////////

func (tangle *Tangle) setTransactionFinalized(transactionID transaction.ID, eventSource EventSource) (modified bool, err error) {
	// retrieve metadata and consume
	cachedTransactionMetadata := tangle.TransactionMetadata(transactionID)
	cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
		// update the finalized flag of the transaction
		modified = metadata.SetFinalized(true)

		// only propagate the changes if the flag was modified
		if modified {
			// retrieve transaction from the database (for the events)
			cachedTransaction := tangle.Transaction(transactionID)
			defer cachedTransaction.Release()
			if !cachedTransaction.Exists() {
				return
			}

			// trigger the corresponding event
			tangle.Events.TransactionFinalized.Trigger(cachedTransaction, cachedTransactionMetadata)

			// propagate the rejected flag
			if !metadata.Preferred() {
				tangle.propagateRejectedToTransactions(metadata.ID())
			}

			// propagate changes to value tangle and branch DAG if we were called from the tangle
			// Note: if the update was triggered by a change in the branch DAG then we do not propagate the confirmed
			//       and rejected changes yet as those require the branch to be liked before (we instead do it in the
			//       BranchLiked event)
			if eventSource == EventSourceTangle {
				// propagate changes to the branches (UTXO DAG)
				if metadata.Conflicting() {
					_, err = tangle.branchManager.SetBranchFinalized(metadata.BranchID())
					if err != nil {
						tangle.Events.Error.Trigger(err)

						return
					}
				}

				// propagate changes to future cone of transaction (value tangle)
				tangle.propagateValuePayloadConfirmedRejectedUpdates(transactionID)
			}
		}
	})

	return
}

// propagateRejectedToTransactions propagates the rejected flag to a transaction, its outputs and to its consumers.
func (tangle *Tangle) propagateRejectedToTransactions(transactionID transaction.ID) {
	// initialize stack with first transaction
	rejectedPropagationStack := list.New()
	rejectedPropagationStack.PushBack(transactionID)

	// keep track of the added transactions so we don't add them multiple times
	addedTransaction := make(map[transaction.ID]types.Empty)

	// work through stack
	for rejectedPropagationStack.Len() >= 1 {
		// pop the first element from the stack
		firstElement := rejectedPropagationStack.Front()
		rejectedPropagationStack.Remove(firstElement)
		currentTransactionID := firstElement.Value.(transaction.ID)

		cachedTransactionMetadata := tangle.TransactionMetadata(currentTransactionID)
		cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			if !metadata.setRejected(true) {
				return
			}

			cachedTransaction := tangle.Transaction(currentTransactionID)
			cachedTransaction.Consume(func(tx *transaction.Transaction) {
				// process all outputs
				tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
					outputID := transaction.NewOutputID(address, currentTransactionID)

					// mark the output to be rejected
					tangle.TransactionOutput(outputID).Consume(func(output *Output) {
						output.setRejected(true)
					})

					// queue consumers to also be rejected
					tangle.Consumers(outputID).Consume(func(consumer *Consumer) {
						if _, transactionAdded := addedTransaction[consumer.TransactionID()]; transactionAdded {
							return
						}
						addedTransaction[consumer.TransactionID()] = types.Void

						rejectedPropagationStack.PushBack(consumer.TransactionID())
					})

					return true
				})

				// trigger event
				tangle.Events.TransactionRejected.Trigger(cachedTransaction, cachedTransactionMetadata)
			})
		})
	}
}

// TODO: WRITE COMMENT
func (tangle *Tangle) propagateValuePayloadConfirmedRejectedUpdates(transactionID transaction.ID) {
	// initiate stack with the attachments of the passed in transaction
	propagationStack := list.New()
	tangle.Attachments(transactionID).Consume(func(attachment *Attachment) {
		propagationStack.PushBack(&valuePayloadPropagationStackEntry{
			CachedPayload:             tangle.Payload(attachment.PayloadID()),
			CachedPayloadMetadata:     tangle.PayloadMetadata(attachment.PayloadID()),
			CachedTransaction:         tangle.Transaction(transactionID),
			CachedTransactionMetadata: tangle.TransactionMetadata(transactionID),
		})
	})

	// keep track of the seen payloads so we do not process them twice
	seenPayloads := make(map[payload.ID]types.Empty)

	// iterate through stack (future cone of transactions)
	for propagationStack.Len() >= 1 {
		currentAttachmentEntry := propagationStack.Front()
		tangle.propagateValuePayloadConfirmedRejectedUpdateStackEntry(propagationStack, seenPayloads, currentAttachmentEntry.Value.(*valuePayloadPropagationStackEntry))
		propagationStack.Remove(currentAttachmentEntry)
	}
}

func (tangle *Tangle) propagateValuePayloadConfirmedRejectedUpdateStackEntry(propagationStack *list.List, processedPayloads map[payload.ID]types.Empty, propagationStackEntry *valuePayloadPropagationStackEntry) {
	// release the entry when we are done
	defer propagationStackEntry.Release()

	// unpack loaded objects and abort if the entities could not be loaded from the database
	currentPayload, currentPayloadMetadata, currentTransaction, currentTransactionMetadata := propagationStackEntry.Unwrap()
	if currentPayload == nil || currentPayloadMetadata == nil || currentTransaction == nil || currentTransactionMetadata == nil {
		return
	}

	// perform different logic depending on the type of the change (liked vs dislike)
	switch currentTransactionMetadata.Preferred() {
	case true:
		// abort if the transaction is not preferred, the branch of the payload is not liked, the referenced value payloads are not liked or the payload was marked as liked before
		if !currentTransactionMetadata.Finalized() || !tangle.BranchManager().IsBranchConfirmed(currentPayloadMetadata.BranchID()) || !tangle.ValuePayloadsConfirmed(currentPayload.TrunkID(), currentPayload.BranchID()) || !currentPayloadMetadata.setConfirmed(true) {
			return
		}

		// trigger payload event
		tangle.Events.PayloadConfirmed.Trigger(propagationStackEntry.CachedPayload, propagationStackEntry.CachedPayloadMetadata)

		// propagate confirmed status to transaction and its outputs
		if currentTransactionMetadata.setConfirmed(true) {
			currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
				tangle.TransactionOutput(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(output *Output) {
					output.setConfirmed(true)
				})

				return true
			})

			tangle.Events.TransactionConfirmed.Trigger(propagationStackEntry.CachedTransaction, propagationStackEntry.CachedTransactionMetadata)
		}
	case false:
		// abort if the payload has been marked as disliked before
		if !currentPayloadMetadata.setRejected(true) {
			return
		}

		tangle.Events.PayloadRejected.Trigger(propagationStackEntry.CachedPayload, propagationStackEntry.CachedPayloadMetadata)
	}

	// schedule checks of approvers and consumers
	tangle.ForEachConsumersAndApprovers(currentPayload, tangle.createValuePayloadFutureConeIterator(propagationStack, processedPayloads))
}

// setTransactionPreferred is an internal utility method that updates the preferred flag and triggers changes to the
// branch DAG and triggers an updates of the liked flags in the value tangle
func (tangle *Tangle) setTransactionPreferred(transactionID transaction.ID, preferred bool, eventSource EventSource) (modified bool, err error) {
	// retrieve metadata and consume
	cachedTransactionMetadata := tangle.TransactionMetadata(transactionID)
	cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
		// update the preferred flag of the transaction
		modified = metadata.setPreferred(preferred)

		// only do something if the flag was modified
		if modified {
			// retrieve transaction from the database (for the events)
			cachedTransaction := tangle.Transaction(transactionID)
			defer cachedTransaction.Release()
			if !cachedTransaction.Exists() {
				return
			}

			// trigger the correct event
			if preferred {
				tangle.Events.TransactionPreferred.Trigger(cachedTransaction, cachedTransactionMetadata)
			} else {
				tangle.Events.TransactionUnpreferred.Trigger(cachedTransaction, cachedTransactionMetadata)
			}

			// propagate changes to value tangle and branch DAG if we were called from the tangle
			// Note: if the update was triggered by a change in the branch DAG then we do not propagate the value
			//       payload changes yet as those require the branch to be liked before (we instead do it in the
			//       BranchLiked event)
			if eventSource == EventSourceTangle {
				// propagate changes to the branches (UTXO DAG)
				if metadata.Conflicting() {
					_, err = tangle.branchManager.SetBranchPreferred(metadata.BranchID(), preferred)
					if err != nil {
						tangle.Events.Error.Trigger(err)

						return
					}
				}

				// propagate changes to future cone of transaction (value tangle)
				tangle.propagateValuePayloadLikeUpdates(transactionID, preferred)
			}
		}
	})

	return
}

// propagateValuePayloadLikeUpdates updates the liked status of all value payloads attaching a certain transaction. If
// the transaction that was updated was the entry point to a branch then all value payloads inside this branch get
// updated as well (updates happen from past to presents).
func (tangle *Tangle) propagateValuePayloadLikeUpdates(transactionID transaction.ID, liked bool) {
	// initiate stack with the attachments of the passed in transaction
	propagationStack := list.New()
	tangle.Attachments(transactionID).Consume(func(attachment *Attachment) {
		propagationStack.PushBack(&valuePayloadPropagationStackEntry{
			CachedPayload:             tangle.Payload(attachment.PayloadID()),
			CachedPayloadMetadata:     tangle.PayloadMetadata(attachment.PayloadID()),
			CachedTransaction:         tangle.Transaction(transactionID),
			CachedTransactionMetadata: tangle.TransactionMetadata(transactionID),
		})
	})

	// keep track of the seen payloads so we do not process them twice
	seenPayloads := make(map[payload.ID]types.Empty)

	// iterate through stack (future cone of transactions)
	for propagationStack.Len() >= 1 {
		currentAttachmentEntry := propagationStack.Front()
		tangle.processValuePayloadLikedUpdateStackEntry(propagationStack, seenPayloads, liked, currentAttachmentEntry.Value.(*valuePayloadPropagationStackEntry))
		propagationStack.Remove(currentAttachmentEntry)
	}
}

// processValuePayloadLikedUpdateStackEntry is an internal utility method that processes a single entry of the
// propagation stack for the update of the liked flag when iterating through the future cone of a transactions
// attachments. It checks if a ValuePayloads has become liked (or disliked), updates the flag an schedules its future
// cone for additional checks.
func (tangle *Tangle) processValuePayloadLikedUpdateStackEntry(propagationStack *list.List, processedPayloads map[payload.ID]types.Empty, liked bool, propagationStackEntry *valuePayloadPropagationStackEntry) {
	// release the entry when we are done
	defer propagationStackEntry.Release()

	// unpack loaded objects and abort if the entities could not be loaded from the database
	currentPayload, currentPayloadMetadata, currentTransaction, currentTransactionMetadata := propagationStackEntry.Unwrap()
	if currentPayload == nil || currentPayloadMetadata == nil || currentTransaction == nil || currentTransactionMetadata == nil {
		return
	}

	// perform different logic depending on the type of the change (liked vs dislike)
	switch liked {
	case true:
		// abort if the transaction is not preferred, the branch of the payload is not liked, the referenced value payloads are not liked or the payload was marked as liked before
		if !currentTransactionMetadata.Preferred() || !tangle.BranchManager().IsBranchLiked(currentPayloadMetadata.BranchID()) || !tangle.ValuePayloadsLiked(currentPayload.TrunkID(), currentPayload.BranchID()) || !currentPayloadMetadata.setLiked(liked) {
			return
		}

		// trigger payload event
		tangle.Events.PayloadLiked.Trigger(propagationStackEntry.CachedPayload, propagationStackEntry.CachedPayloadMetadata)

		// propagate liked to transaction and its outputs
		if currentTransactionMetadata.setLiked(true) {
			currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
				tangle.TransactionOutput(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(output *Output) {
					output.setLiked(true)
				})

				return true
			})

			// trigger event
			tangle.Events.TransactionLiked.Trigger(propagationStackEntry.CachedTransaction, propagationStackEntry.CachedTransactionMetadata)
		}
	case false:
		// abort if the payload has been marked as disliked before
		if !currentPayloadMetadata.setLiked(liked) {
			return
		}

		tangle.Events.PayloadDisliked.Trigger(propagationStackEntry.CachedPayload, propagationStackEntry.CachedPayloadMetadata)

		// look if we still have any liked attachments of this transaction
		likedAttachmentFound := false
		tangle.Attachments(currentTransaction.ID()).Consume(func(attachment *Attachment) {
			tangle.PayloadMetadata(attachment.PayloadID()).Consume(func(payloadMetadata *PayloadMetadata) {
				likedAttachmentFound = likedAttachmentFound || payloadMetadata.Liked()
			})
		})

		// if there are no other liked attachments of this transaction then also set it to disliked
		if !likedAttachmentFound {
			// propagate disliked to transaction and its outputs
			if currentTransactionMetadata.setLiked(false) {
				currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
					tangle.TransactionOutput(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(output *Output) {
						output.setLiked(false)
					})

					return true
				})

				// trigger event
				tangle.Events.TransactionDisliked.Trigger(propagationStackEntry.CachedTransaction, propagationStackEntry.CachedTransactionMetadata)
			}
		}
	}

	// schedule checks of approvers and consumers
	tangle.ForEachConsumersAndApprovers(currentPayload, tangle.createValuePayloadFutureConeIterator(propagationStack, processedPayloads))
}

// createValuePayloadFutureConeIterator returns a function that can be handed into the ForEachConsumersAndApprovers
// method, that iterates through the next level of the future cone of the given transaction and adds the found elements
// to the given stack.
func (tangle *Tangle) createValuePayloadFutureConeIterator(propagationStack *list.List, processedPayloads map[payload.ID]types.Empty) func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
	return func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
		// automatically release cached objects when we terminate
		defer cachedPayload.Release()
		defer cachedPayloadMetadata.Release()
		defer cachedTransaction.Release()
		defer cachedTransactionMetadata.Release()

		// abort if the payload could not be unwrapped
		unwrappedPayload := cachedPayload.Unwrap()
		if unwrappedPayload == nil {
			return
		}

		// abort if we have scheduled the check of this payload already
		if _, payloadProcessedAlready := processedPayloads[unwrappedPayload.ID()]; payloadProcessedAlready {
			return
		}
		processedPayloads[unwrappedPayload.ID()] = types.Void

		// schedule next checks
		propagationStack.PushBack(&valuePayloadPropagationStackEntry{
			CachedPayload:             cachedPayload.Retain(),
			CachedPayloadMetadata:     cachedPayloadMetadata.Retain(),
			CachedTransaction:         cachedTransaction.Retain(),
			CachedTransactionMetadata: cachedTransactionMetadata.Retain(),
		})
	}
}

func (tangle *Tangle) storePayload(payloadToStore *payload.Payload) (cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, payloadStored bool) {
	storedTransaction, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore)
	if !transactionIsNew {
		return
	}

	cachedPayload = &payload.CachedPayload{CachedObject: storedTransaction}
	cachedMetadata = &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadToStore.ID()))}
	payloadStored = true

	return
}

func (tangle *Tangle) storeTransactionModels(solidPayload *payload.Payload) (cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment, transactionIsNew bool) {
	cachedTransaction = &transaction.CachedTransaction{CachedObject: tangle.transactionStorage.ComputeIfAbsent(solidPayload.Transaction().ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionIsNew = true

		result := solidPayload.Transaction()
		result.Persist()
		result.SetModified()

		return result
	})}

	if transactionIsNew {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Store(NewTransactionMetadata(solidPayload.Transaction().ID()))}

		// store references to the consumed outputs
		solidPayload.Transaction().Inputs().ForEach(func(outputId transaction.OutputID) bool {
			tangle.consumerStorage.Store(NewConsumer(outputId, solidPayload.Transaction().ID())).Release()

			return true
		})
	} else {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(solidPayload.Transaction().ID().Bytes())}
	}

	// store a reference from the transaction to the payload that attached it or abort, if we have processed this attachment already
	attachment, stored := tangle.attachmentStorage.StoreIfAbsent(NewAttachment(solidPayload.Transaction().ID(), solidPayload.ID()))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}

	return
}

func (tangle *Tangle) storePayloadReferences(payload *payload.Payload) {
	// store trunk approver
	trunkID := payload.TrunkID()
	tangle.approverStorage.Store(NewPayloadApprover(trunkID, payload.ID())).Release()

	// store branch approver
	if branchID := payload.BranchID(); branchID != trunkID {
		tangle.approverStorage.Store(NewPayloadApprover(branchID, payload.ID())).Release()
	}
}

// solidifyPayload is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyPayload(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack(&valuePayloadPropagationStackEntry{
		CachedPayload:             cachedPayload,
		CachedPayloadMetadata:     cachedMetadata,
		CachedTransaction:         cachedTransaction,
		CachedTransactionMetadata: cachedTransactionMetadata,
	})

	// keep track of the added payloads so we do not add them multiple times
	processedPayloads := make(map[payload.ID]types.Empty)

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentSolidificationEntry := solidificationStack.Front()
		tangle.processSolidificationStackEntry(solidificationStack, processedPayloads, currentSolidificationEntry.Value.(*valuePayloadPropagationStackEntry))
		solidificationStack.Remove(currentSolidificationEntry)
	}
}

// processSolidificationStackEntry processes a single entry of the solidification stack and schedules its approvers and
// consumers if necessary.
func (tangle *Tangle) processSolidificationStackEntry(solidificationStack *list.List, processedPayloads map[payload.ID]types.Empty, solidificationStackEntry *valuePayloadPropagationStackEntry) {
	// release stack entry when we are done
	defer solidificationStackEntry.Release()

	// unwrap and abort if any of the retrieved models are nil
	currentPayload, currentPayloadMetadata, currentTransaction, currentTransactionMetadata := solidificationStackEntry.Unwrap()
	if currentPayload == nil || currentPayloadMetadata == nil || currentTransaction == nil || currentTransactionMetadata == nil {
		return
	}

	// abort if the transaction is not solid or invalid
	transactionSolid, consumedBranches, transactionSolidityErr := tangle.checkTransactionSolidity(currentTransaction, currentTransactionMetadata)
	if transactionSolidityErr != nil {
		// TODO: TRIGGER INVALID TX + REMOVE TXS + PAYLOADS THAT APPROVE IT

		return
	}
	if !transactionSolid {
		return
	}

	// abort if the payload is not solid or invalid
	payloadSolid, payloadSolidityErr := tangle.checkPayloadSolidity(currentPayload, currentPayloadMetadata, consumedBranches)
	if payloadSolidityErr != nil {
		// TODO: TRIGGER INVALID TX + REMOVE TXS + PAYLOADS THAT APPROVE IT

		return
	}
	if !payloadSolid {
		return
	}

	// book the solid entities
	transactionBooked, payloadBooked, decisionPending, bookingErr := tangle.book(solidificationStackEntry.Retain())
	if bookingErr != nil {
		tangle.Events.Error.Trigger(bookingErr)

		return
	}

	// trigger events and schedule check of approvers / consumers
	if transactionBooked {
		tangle.Events.TransactionBooked.Trigger(solidificationStackEntry.CachedTransaction, solidificationStackEntry.CachedTransactionMetadata, decisionPending)

		tangle.ForEachConsumers(currentTransaction, tangle.createValuePayloadFutureConeIterator(solidificationStack, processedPayloads))
	}
	if payloadBooked {
		tangle.ForeachApprovers(currentPayload.ID(), tangle.createValuePayloadFutureConeIterator(solidificationStack, processedPayloads))
	}
}

func (tangle *Tangle) book(entitiesToBook *valuePayloadPropagationStackEntry) (transactionBooked bool, payloadBooked bool, decisionPending bool, err error) {
	defer entitiesToBook.Release()

	if transactionBooked, decisionPending, err = tangle.bookTransaction(entitiesToBook.CachedTransaction.Retain(), entitiesToBook.CachedTransactionMetadata.Retain()); err != nil {
		return
	}

	if payloadBooked, err = tangle.bookPayload(entitiesToBook.CachedPayload.Retain(), entitiesToBook.CachedPayloadMetadata.Retain(), entitiesToBook.CachedTransactionMetadata.Retain()); err != nil {
		return
	}

	return
}

func (tangle *Tangle) bookTransaction(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) (transactionBooked bool, decisionPending bool, err error) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	transactionToBook := cachedTransaction.Unwrap()
	if transactionToBook == nil {
		// TODO: explicit error var
		err = errors.New("failed to unwrap transaction")

		return
	}

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		// TODO: explicit error var
		err = errors.New("failed to unwrap transaction metadata")

		return
	}

	// abort if this transaction was booked by another process already
	if !transactionMetadata.SetSolid(true) {
		return
	}

	consumedBranches := make(branchmanager.BranchIds)
	conflictingInputs := make([]transaction.OutputID, 0)
	conflictingInputsOfFirstConsumers := make(map[transaction.ID][]transaction.OutputID)

	if !transactionToBook.Inputs().ForEach(func(outputID transaction.OutputID) bool {
		cachedOutput := tangle.TransactionOutput(outputID)
		defer cachedOutput.Release()

		// abort if the output could not be found
		output := cachedOutput.Unwrap()
		if output == nil {
			err = fmt.Errorf("could not load output '%s'", outputID)

			return false
		}

		consumedBranches[output.BranchID()] = types.Void

		// register the current consumer and check if the input has been consumed before
		consumerCount, firstConsumerID := output.RegisterConsumer(transactionToBook.ID())
		switch consumerCount {
		// continue if we are the first consumer and there is no double spend
		case 0:
			return true

		// if the input has been consumed before but not been forked, yet
		case 1:
			// keep track of the conflicting inputs so we can fork them
			if _, conflictingInputsExist := conflictingInputsOfFirstConsumers[firstConsumerID]; !conflictingInputsExist {
				conflictingInputsOfFirstConsumers[firstConsumerID] = make([]transaction.OutputID, 0)
			}
			conflictingInputsOfFirstConsumers[firstConsumerID] = append(conflictingInputsOfFirstConsumers[firstConsumerID], outputID)
		}

		// mark input as conflicting
		conflictingInputs = append(conflictingInputs, outputID)

		return true
	}) {
		return
	}

	cachedTargetBranch, err := tangle.branchManager.AggregateBranches(consumedBranches.ToList()...)
	if err != nil {
		return
	}
	defer cachedTargetBranch.Release()

	targetBranch := cachedTargetBranch.Unwrap()
	if targetBranch == nil {
		err = errors.New("failed to unwrap target branch")

		return
	}
	targetBranch.Persist()

	if len(conflictingInputs) >= 1 {
		cachedTargetBranch, _ = tangle.branchManager.Fork(branchmanager.NewBranchID(transactionToBook.ID()), []branchmanager.BranchID{targetBranch.ID()}, conflictingInputs)
		defer cachedTargetBranch.Release()

		targetBranch = cachedTargetBranch.Unwrap()
		if targetBranch == nil {
			err = errors.New("failed to inherit branches")

			return
		}
	}

	// book transaction into target branch
	transactionMetadata.SetBranchID(targetBranch.ID())

	// create color for newly minted coins
	mintedColor, _, err := balance.ColorFromBytes(transactionToBook.ID().Bytes())
	if err != nil {
		panic(err) // this should never happen (a transaction id is always a valid color)
	}

	// book outputs into the target branch
	transactionToBook.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		// create correctly colored balances (replacing color of newly minted coins with color of transaction id)
		coloredBalances := make([]*balance.Balance, len(balances))
		for i, currentBalance := range balances {
			if currentBalance.Color() == balance.ColorNew {
				coloredBalances[i] = balance.New(mintedColor, currentBalance.Value())
			} else {
				coloredBalances[i] = currentBalance
			}
		}

		// store output
		newOutput := NewOutput(address, transactionToBook.ID(), targetBranch.ID(), coloredBalances)
		newOutput.setSolid(true)
		tangle.outputStorage.Store(newOutput).Release()

		return true
	})

	// fork the conflicting transactions into their own branch
	for consumerID, conflictingInputs := range conflictingInputsOfFirstConsumers {
		_, decisionFinalized, forkedErr := tangle.Fork(consumerID, conflictingInputs)
		if forkedErr != nil {
			err = forkedErr

			return
		}

		decisionPending = decisionPending || !decisionFinalized
	}
	transactionBooked = true

	return
}

func (tangle *Tangle) bookPayload(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) (payloadBooked bool, err error) {
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()
	defer cachedTransactionMetadata.Release()

	valueObject := cachedPayload.Unwrap()
	valueObjectMetadata := cachedPayloadMetadata.Unwrap()
	transactionMetadata := cachedTransactionMetadata.Unwrap()

	if valueObject == nil || valueObjectMetadata == nil || transactionMetadata == nil {
		return
	}

	branchBranchID := tangle.payloadBranchID(valueObject.BranchID())
	trunkBranchID := tangle.payloadBranchID(valueObject.TrunkID())
	transactionBranchID := transactionMetadata.BranchID()

	if branchBranchID == branchmanager.UndefinedBranchID || trunkBranchID == branchmanager.UndefinedBranchID || transactionBranchID == branchmanager.UndefinedBranchID {
		return
	}

	cachedAggregatedBranch, err := tangle.BranchManager().AggregateBranches([]branchmanager.BranchID{branchBranchID, trunkBranchID, transactionBranchID}...)
	if err != nil {
		return
	}
	defer cachedAggregatedBranch.Release()

	aggregatedBranch := cachedAggregatedBranch.Unwrap()
	if aggregatedBranch == nil {
		return
	}

	payloadBooked = valueObjectMetadata.SetBranchID(aggregatedBranch.ID())

	return
}

// payloadBranchID returns the BranchID that the referenced Payload was booked into.
func (tangle *Tangle) payloadBranchID(payloadID payload.ID) branchmanager.BranchID {
	if payloadID == payload.GenesisID {
		return branchmanager.MasterBranchID
	}

	cachedPayloadMetadata := tangle.PayloadMetadata(payloadID)
	defer cachedPayloadMetadata.Release()

	payloadMetadata := cachedPayloadMetadata.Unwrap()
	if payloadMetadata == nil {
		cachedPayloadMetadata.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(NewMissingPayload(payloadID)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*MissingPayload).ID())
			})
		}

		return branchmanager.UndefinedBranchID
	}

	// the BranchID is only set if the payload was also marked as solid
	return payloadMetadata.BranchID()
}

// checkPayloadSolidity returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) checkPayloadSolidity(payload *payload.Payload, payloadMetadata *PayloadMetadata, transactionBranches []branchmanager.BranchID) (solid bool, err error) {
	if payload == nil || payload.IsDeleted() || payloadMetadata == nil || payloadMetadata.IsDeleted() {
		return
	}

	if solid = payloadMetadata.IsSolid(); solid {
		return
	}

	combinedBranches := transactionBranches

	trunkBranchID := tangle.payloadBranchID(payload.TrunkID())
	if trunkBranchID == branchmanager.UndefinedBranchID {
		return
	}
	combinedBranches = append(combinedBranches, trunkBranchID)

	branchBranchID := tangle.payloadBranchID(payload.BranchID())
	if branchBranchID == branchmanager.UndefinedBranchID {
		return
	}
	combinedBranches = append(combinedBranches, branchBranchID)

	branchesConflicting, err := tangle.branchManager.BranchesConflicting(combinedBranches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the payload '%s' combines conflicting versions of the ledger state", payload.ID())

		return
	}

	solid = true

	return
}

func (tangle *Tangle) checkTransactionSolidity(tx *transaction.Transaction, metadata *TransactionMetadata) (solid bool, consumedBranches []branchmanager.BranchID, err error) {
	// abort if any of the models are nil or has been deleted
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return
	}

	// abort if we have previously determined the solidity status of the transaction already
	if solid = metadata.Solid(); solid {
		consumedBranches = []branchmanager.BranchID{metadata.BranchID()}

		return
	}

	// determine the consumed inputs and balances of the transaction
	inputsSolid, cachedInputs, consumedBalances, consumedBranchesMap, err := tangle.retrieveConsumedInputDetails(tx)
	if err != nil || !inputsSolid {
		return
	}
	defer cachedInputs.Release()

	// abort if the outputs are not matching the inputs
	if !tangle.checkTransactionOutputs(consumedBalances, tx.Outputs()) {
		err = fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.ID())

		return
	}

	// abort if the branches are conflicting or we faced an error when checking the validity
	consumedBranches = consumedBranchesMap.ToList()
	branchesConflicting, err := tangle.branchManager.BranchesConflicting(consumedBranches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the transaction '%s' spends conflicting inputs", tx.ID())

		return
	}

	// set the result to be solid and valid
	solid = true

	return
}

func (tangle *Tangle) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result CachedOutputs) {
	result = make(CachedOutputs)
	tx.Inputs().ForEach(func(inputId transaction.OutputID) bool {
		result[inputId] = tangle.TransactionOutput(inputId)

		return true
	})

	return
}

func (tangle *Tangle) retrieveConsumedInputDetails(tx *transaction.Transaction) (inputsSolid bool, cachedInputs CachedOutputs, consumedBalances map[balance.Color]int64, consumedBranches branchmanager.BranchIds, err error) {
	cachedInputs = tangle.getCachedOutputsFromTransactionInputs(tx)
	consumedBalances = make(map[balance.Color]int64)
	consumedBranches = make(branchmanager.BranchIds)
	for _, cachedInput := range cachedInputs {
		input := cachedInput.Unwrap()
		if input == nil || !input.Solid() {
			cachedInputs.Release()

			return
		}

		consumedBranches[input.BranchID()] = types.Void

		// calculate the input balances
		for _, inputBalance := range input.Balances() {
			var newBalance int64
			if currentBalance, balanceExists := consumedBalances[inputBalance.Color()]; balanceExists {
				// check overflows in the numbers
				if inputBalance.Value() > math.MaxInt64-currentBalance {
					// TODO: make it an explicit error var
					err = fmt.Errorf("buffer overflow in balances of inputs")

					cachedInputs.Release()

					return
				}

				newBalance = currentBalance + inputBalance.Value()
			} else {
				newBalance = inputBalance.Value()
			}
			consumedBalances[inputBalance.Color()] = newBalance
		}
	}
	inputsSolid = true

	return
}

// checkTransactionOutputs is a utility function that returns true, if the outputs are consuming all of the given inputs
// (the sum of all the balance changes is 0). It also accounts for the ability to "recolor" coins during the creating of
// outputs. If this function returns false, then the outputs that are defined in the transaction are invalid and the
// transaction should be removed from the ledger state.
func (tangle *Tangle) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
	// create a variable to keep track of outputs that create a new color
	var newlyColoredCoins int64
	var uncoloredCoins int64

	// iterate through outputs and check them one by one
	aborted := !outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		for _, outputBalance := range balances {
			// abort if the output creates a negative or empty output
			if outputBalance.Value() <= 0 {
				return false
			}

			// sidestep logic if we have a newly colored output (we check the supply later)
			if outputBalance.Color() == balance.ColorNew {
				// catch overflows
				if newlyColoredCoins > math.MaxInt64-outputBalance.Value() {
					return false
				}

				newlyColoredCoins += outputBalance.Value()

				continue
			}

			// sidestep logic if we have a newly colored output (we check the supply later)
			if outputBalance.Color() == balance.ColorIOTA {
				// catch overflows
				if uncoloredCoins > math.MaxInt64-outputBalance.Value() {
					return false
				}

				uncoloredCoins += outputBalance.Value()

				continue
			}

			// check if the used color does not exist in our supply
			availableBalance, spentColorExists := inputBalances[outputBalance.Color()]
			if !spentColorExists {
				return false
			}

			// abort if we spend more coins of the given color than we have
			if availableBalance < outputBalance.Value() {
				return false
			}

			// subtract the spent coins from the supply of this color
			inputBalances[outputBalance.Color()] -= outputBalance.Value()

			// cleanup empty map entries (we have exhausted our funds)
			if inputBalances[outputBalance.Color()] == 0 {
				delete(inputBalances, outputBalance.Color())
			}
		}

		return true
	})

	// abort if the previous checks failed
	if aborted {
		return false
	}

	// determine the unspent inputs
	var unspentCoins int64
	for _, unspentBalance := range inputBalances {
		// catch overflows
		if unspentCoins > math.MaxInt64-unspentBalance {
			return false
		}

		unspentCoins += unspentBalance
	}

	// the outputs are valid if they spend all consumed funds
	return unspentCoins == newlyColoredCoins+uncoloredCoins
}

// TODO: write comment what it does
func (tangle *Tangle) moveTransactionToBranch(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedTargetBranch *branchmanager.CachedBranch) (err error) {
	// push transaction that shall be moved to the stack
	transactionStack := list.New()
	branchStack := list.New()
	branchStack.PushBack([3]interface{}{cachedTransactionMetadata.Unwrap().BranchID(), cachedTargetBranch, transactionStack})
	transactionStack.PushBack([2]interface{}{cachedTransaction, cachedTransactionMetadata})

	// iterate through all transactions (grouped by their branch)
	for branchStack.Len() >= 1 {
		if err = func() error {
			// retrieve branch details from stack
			currentSolidificationEntry := branchStack.Front()
			currentSourceBranch := currentSolidificationEntry.Value.([3]interface{})[0].(branchmanager.BranchID)
			currentCachedTargetBranch := currentSolidificationEntry.Value.([3]interface{})[1].(*branchmanager.CachedBranch)
			transactionStack := currentSolidificationEntry.Value.([3]interface{})[2].(*list.List)
			branchStack.Remove(currentSolidificationEntry)
			defer currentCachedTargetBranch.Release()

			// unpack target branch
			targetBranch := currentCachedTargetBranch.Unwrap()
			if targetBranch == nil {
				return errors.New("failed to unpack branch")
			}

			// iterate through transactions
			for transactionStack.Len() >= 1 {
				if err = func() error {
					// retrieve transaction details from stack
					currentSolidificationEntry := transactionStack.Front()
					currentCachedTransaction := currentSolidificationEntry.Value.([2]interface{})[0].(*transaction.CachedTransaction)
					currentCachedTransactionMetadata := currentSolidificationEntry.Value.([2]interface{})[1].(*CachedTransactionMetadata)
					transactionStack.Remove(currentSolidificationEntry)
					defer currentCachedTransaction.Release()
					defer currentCachedTransactionMetadata.Release()

					// unwrap transaction
					currentTransaction := currentCachedTransaction.Unwrap()
					if currentTransaction == nil {
						return errors.New("failed to unwrap transaction")
					}

					// unwrap transaction metadata
					currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
					if currentTransactionMetadata == nil {
						return errors.New("failed to unwrap transaction metadata")
					}

					// if we arrived at a nested branch
					if currentTransactionMetadata.BranchID() != currentSourceBranch {
						// abort if we the branch is a conflict branch or an error occurred while trying to elevate
						isConflictBranch, _, elevateErr := tangle.branchManager.ElevateConflictBranch(currentTransactionMetadata.BranchID(), targetBranch.ID())
						if elevateErr != nil || isConflictBranch {
							return elevateErr
						}

						// determine the new branch of the transaction
						newCachedTargetBranch, branchErr := tangle.calculateBranchOfTransaction(currentTransaction)
						if branchErr != nil {
							return branchErr
						}
						defer newCachedTargetBranch.Release()

						// unwrap the branch
						newTargetBranch := newCachedTargetBranch.Unwrap()
						if newTargetBranch == nil {
							return errors.New("failed to unwrap branch")
						}
						newTargetBranch.Persist()

						// add the new branch (with the current transaction as a starting point to the branch stack)
						newTransactionStack := list.New()
						newTransactionStack.PushBack([2]interface{}{currentCachedTransaction.Retain(), currentCachedTransactionMetadata.Retain()})
						branchStack.PushBack([3]interface{}{currentTransactionMetadata.BranchID(), newCachedTargetBranch.Retain(), newTransactionStack})

						return nil
					}

					// abort if we did not modify the branch of the transaction
					if !currentTransactionMetadata.SetBranchID(targetBranch.ID()) {
						return nil
					}

					// iterate through the outputs of the moved transaction
					currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
						// create reference to the output
						outputID := transaction.NewOutputID(address, currentTransaction.ID())

						// load output from database
						cachedOutput := tangle.TransactionOutput(outputID)
						defer cachedOutput.Release()

						// unwrap output
						output := cachedOutput.Unwrap()
						if output == nil {
							err = fmt.Errorf("failed to load output '%s'", outputID)

							return false
						}

						// abort if the output was moved already
						if !output.SetBranchID(targetBranch.ID()) {
							return true
						}

						// schedule consumers for further checks
						consumingTransactions := make(map[transaction.ID]types.Empty)
						tangle.Consumers(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(consumer *Consumer) {
							consumingTransactions[consumer.TransactionID()] = types.Void
						})
						for transactionID := range consumingTransactions {
							transactionStack.PushBack([2]interface{}{tangle.Transaction(transactionID), tangle.TransactionMetadata(transactionID)})
						}

						return true
					})

					return nil
				}(); err != nil {
					return err
				}
			}

			return nil
		}(); err != nil {
			return
		}
	}

	return
}

func (tangle *Tangle) calculateBranchOfTransaction(currentTransaction *transaction.Transaction) (branch *branchmanager.CachedBranch, err error) {
	consumedBranches := make(branchmanager.BranchIds)
	if !currentTransaction.Inputs().ForEach(func(outputId transaction.OutputID) bool {
		cachedTransactionOutput := tangle.TransactionOutput(outputId)
		defer cachedTransactionOutput.Release()

		transactionOutput := cachedTransactionOutput.Unwrap()
		if transactionOutput == nil {
			err = fmt.Errorf("failed to load output '%s'", outputId)

			return false
		}

		consumedBranches[transactionOutput.BranchID()] = types.Void

		return true
	}) {
		return
	}

	branch, err = tangle.branchManager.AggregateBranches(consumedBranches.ToList()...)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// valuePayloadPropagationStackEntry is a container for the elements in the propagation stack of ValuePayloads
type valuePayloadPropagationStackEntry struct {
	CachedPayload             *payload.CachedPayload
	CachedPayloadMetadata     *CachedPayloadMetadata
	CachedTransaction         *transaction.CachedTransaction
	CachedTransactionMetadata *CachedTransactionMetadata
}

// Retain creates a new container with its contained elements being retained for further use.
func (stackEntry *valuePayloadPropagationStackEntry) Retain() *valuePayloadPropagationStackEntry {
	return &valuePayloadPropagationStackEntry{
		CachedPayload:             stackEntry.CachedPayload.Retain(),
		CachedPayloadMetadata:     stackEntry.CachedPayloadMetadata.Retain(),
		CachedTransaction:         stackEntry.CachedTransaction.Retain(),
		CachedTransactionMetadata: stackEntry.CachedTransactionMetadata.Retain(),
	}
}

// Release releases the elements in this container for being written by the objectstorage.
func (stackEntry *valuePayloadPropagationStackEntry) Release() {
	stackEntry.CachedPayload.Release()
	stackEntry.CachedPayloadMetadata.Release()
	stackEntry.CachedTransaction.Release()
	stackEntry.CachedTransactionMetadata.Release()
}

// Unwrap retrieves the underlying StorableObjects from the cached elements in this container.
func (stackEntry *valuePayloadPropagationStackEntry) Unwrap() (payload *payload.Payload, payloadMetadata *PayloadMetadata, transaction *transaction.Transaction, transactionMetadata *TransactionMetadata) {
	payload = stackEntry.CachedPayload.Unwrap()
	payloadMetadata = stackEntry.CachedPayloadMetadata.Unwrap()
	transaction = stackEntry.CachedTransaction.Unwrap()
	transactionMetadata = stackEntry.CachedTransactionMetadata.Unwrap()

	return
}
