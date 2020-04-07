package tangle

import (
	"container/list"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingPayloadStorage  *objectstorage.ObjectStorage
	attachmentStorage      *objectstorage.ObjectStorage

	outputStorage        *objectstorage.ObjectStorage
	consumerStorage      *objectstorage.ObjectStorage
	missingOutputStorage *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		// payload related storage
		payloadStorage:         osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:        osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),

		// transaction related storage
		attachmentStorage:    osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second)),
		outputStorage:        osFactory.New(osOutput, osOutputFactory, transaction.OutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		missingOutputStorage: osFactory.New(osMissingOutput, osMissingOutputFactory, MissingOutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		consumerStorage:      osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second)),

		Events: *newEvents(),
	}

	return
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// GetPayload retrieves a payload from the object storage.
func (tangle *Tangle) GetPayload(payloadId payload.Id) *payload.CachedPayload {
	return &payload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetPayloadMetadata(payloadId payload.Id) *CachedPayloadMetadata {
	return &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetTransactionMetadata(transactionId transaction.Id) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: tangle.missingOutputStorage.Load(transactionId.Bytes())}
}

func (tangle *Tangle) GetTransactionOutput(outputId transaction.OutputId) *transaction.CachedOutput {
	return &transaction.CachedOutput{CachedObject: tangle.outputStorage.Load(outputId.Bytes())}
}

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetApprovers(payloadId payload.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, payloadId.Bytes())

	return approvers
}

// GetConsumers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetConsumers(outputId transaction.OutputId) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	tangle.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputId.Bytes())

	return consumers
}

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetAttachments(transactionId transaction.Id) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	tangle.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionId.Bytes())

	return attachments
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storePayloadWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	tangle.payloadStorage.Shutdown()
	tangle.payloadMetadataStorage.Shutdown()
	tangle.approverStorage.Shutdown()
	tangle.missingPayloadStorage.Shutdown()

	return tangle
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (tangle *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.approverStorage,
		tangle.missingPayloadStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payloadToStore *payload.Payload) {
	// store the payload and transaction models
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		// abort if we have seen the payload already
		return
	}
	cachedTransactionMetadata, transactionStored := tangle.storeTransaction(payloadToStore.Transaction())

	// store the references between the different entities (we do this after the actual entities were stored, so that
	// all the metadata models exist in the database as soon as the entities are reachable by walks).
	tangle.storePayloadReferences(payloadToStore)
	if transactionStored {
		tangle.storeTransactionReferences(payloadToStore.Transaction())
	}

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadToStore.Id().Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedPayloadMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyTransactionWorker(cachedPayload, cachedPayloadMetadata, cachedTransactionMetadata)
	})
}

func (tangle *Tangle) storePayload(payloadToStore *payload.Payload) (cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, payloadStored bool) {
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore); !transactionIsNew {
		return
	} else {
		cachedPayload = &payload.CachedPayload{CachedObject: _tmp}
		cachedMetadata = &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadToStore.Id()))}
		payloadStored = true

		return
	}
}

func (tangle *Tangle) storeTransaction(tx *transaction.Transaction) (cachedTransactionMetadata *CachedTransactionMetadata, transactionStored bool) {
	cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.payloadMetadataStorage.ComputeIfAbsent(tx.Id().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionStored = true

		result := NewTransactionMetadata(tx.Id())
		result.Persist()
		result.SetModified()

		return result
	})}

	if transactionStored {
		tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			tangle.outputStorage.Store(transaction.NewOutput(address, tx.Id(), balances))

			return true
		})
	}

	return
}

func (tangle *Tangle) storePayloadReferences(payload *payload.Payload) {
	// store trunk approver
	trunkId := payload.TrunkId()
	tangle.approverStorage.Store(NewPayloadApprover(trunkId, payload.Id())).Release()

	// store branch approver
	if branchId := payload.BranchId(); branchId != trunkId {
		tangle.approverStorage.Store(NewPayloadApprover(branchId, trunkId)).Release()
	}

	// store a reference from the transaction to the payload that attached it
	tangle.attachmentStorage.Store(NewAttachment(payload.Transaction().Id(), payload.Id()))
}

func (tangle *Tangle) storeTransactionReferences(tx *transaction.Transaction) {
	// store references to the consumed outputs
	tx.Inputs().ForEach(func(outputId transaction.OutputId) bool {
		tangle.consumerStorage.Store(NewConsumer(outputId, tx.Id()))

		return true
	})
}

// solidifyTransactionWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyTransactionWorker(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
	popElementsFromStack := func(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata, *CachedTransactionMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedPayload := currentSolidificationEntry.Value.([3]interface{})[0]
		currentCachedMetadata := currentSolidificationEntry.Value.([3]interface{})[1]
		currentCachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[2]
		stack.Remove(currentSolidificationEntry)

		return currentCachedPayload.(*payload.CachedPayload), currentCachedMetadata.(*CachedPayloadMetadata), currentCachedTransactionMetadata.(*CachedTransactionMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedPayload, cachedMetadata, cachedTransactionMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedPayload, currentCachedMetadata, currentCachedTransactionMetadata := popElementsFromStack(solidificationStack)
			defer currentCachedPayload.Release()
			defer currentCachedMetadata.Release()
			defer currentCachedTransactionMetadata.Release()

			// unwrap cached objects
			currentPayload := currentCachedPayload.Unwrap()
			currentPayloadMetadata := currentCachedMetadata.Unwrap()
			currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
			currentTransaction := currentPayload.Transaction()

			// abort if any of the retrieved models is nil
			if currentPayload == nil || currentPayloadMetadata == nil || currentTransactionMetadata == nil {
				return
			}

			// abort if the payload is not solid
			if !tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) {
				return
			}

			// abort if the transaction is not solid or invalid
			if transactionSolid, err := tangle.isTransactionSolid(currentTransaction, currentTransactionMetadata); !transactionSolid || err != nil {
				if err != nil {
					// TODO: TRIGGER INVALID TX + REMOVE TXS THAT APPROVE IT
					fmt.Println(err)
				}

				return
			}

			// abort if the payload was marked as solid already (if a payload is solid already then the tx is also solid)
			if !currentPayloadMetadata.SetSolid(true) {
				return
			}

			// set the transaction related entities to be solid
			transactionBecameSolid := currentTransactionMetadata.SetSolid(true)
			if transactionBecameSolid {
				currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
					tangle.GetTransactionOutput(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(output *transaction.Output) {
						output.SetSolid(true)
					})

					return true
				})
			}

			// ... trigger solid event ...
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			// ... and schedule check of approvers
			tangle.GetApprovers(currentPayload.Id()).Consume(func(approver *PayloadApprover) {
				approvingPayloadId := approver.GetApprovingPayloadId()
				approvingCachedPayload := tangle.GetPayload(approvingPayloadId)

				approvingCachedPayload.Consume(func(payload *payload.Payload) {
					solidificationStack.PushBack([3]interface{}{
						approvingCachedPayload,
						tangle.GetPayloadMetadata(approvingPayloadId),
						tangle.GetTransactionMetadata(payload.Transaction().Id()),
					})
				})
			})

			if !transactionBecameSolid {
				return
			}

			tangle.Events.TransactionSolid.Trigger(currentTransaction, currentTransactionMetadata)

			seenTransactions := make(map[transaction.Id]types.Empty)
			currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
				tangle.GetTransactionOutput(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(output *transaction.Output) {
					// trigger events
				})

				tangle.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
					// keep track of the processed transactions (the same transaction can consume multiple outputs)
					if _, transactionSeen := seenTransactions[consumer.TransactionId()]; transactionSeen {
						seenTransactions[consumer.TransactionId()] = types.Void

						transactionMetadata := tangle.GetTransactionMetadata(consumer.TransactionId())

						// retrieve all the payloads that attached the transaction
						tangle.GetAttachments(consumer.TransactionId()).Consume(func(attachment *Attachment) {
							solidificationStack.PushBack([3]interface{}{
								tangle.GetPayload(attachment.PayloadId()),
								tangle.GetPayloadMetadata(attachment.PayloadId()),
								transactionMetadata,
							})
						})
					}
				})

				return true
			})
		}()
	}
}

func (tangle *Tangle) isTransactionSolid(tx *transaction.Transaction, metadata *TransactionMetadata) (bool, error) {
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return false, nil
	}

	if metadata.Solid() {
		return true, nil
	}

	// get outputs that were referenced in the transaction inputs
	cachedInputs := tangle.getCachedOutputsFromTransactionInputs(tx)
	defer func() {
		for _, input := range cachedInputs {
			input.Release()
		}
	}()

	// iterate through the inputs to see if they exist, are solid and to calculate the sum of their balances
	inputsSolid := true
	availableBalances := make(map[balance.Color]int64)
	for inputId, cachedInput := range cachedInputs {
		if !cachedInput.Exists() {
			inputsSolid = false

			if cachedMissingOutput, missingOutputStored := tangle.missingOutputStorage.StoreIfAbsent(NewMissingOutput(inputId)); missingOutputStored {
				cachedMissingOutput.Consume(func(object objectstorage.StorableObject) {
					tangle.Events.OutputMissing.Trigger(object.(*MissingOutput).Id())
				})
			}

			continue
		}

		// should never be nil as we check Exists() before
		input := cachedInput.Unwrap()

		// update solid status
		inputsSolid = inputsSolid && input.Solid()

		// calculate the input balances
		for _, inputBalance := range input.Balances() {
			var newBalance int64
			if currentBalance, balanceExists := availableBalances[inputBalance.Color()]; balanceExists {
				// check overflows in the numbers
				if inputBalance.Value() > math.MaxInt64-currentBalance {
					return false, fmt.Errorf("buffer overflow in inputs of transaction '%s'", tx.Id())
				}

				newBalance = currentBalance + inputBalance.Value()
			} else {
				newBalance = inputBalance.Value()
			}
			availableBalances[inputBalance.Color()] = newBalance
		}
	}

	// abort if the inputs are not solid
	if !inputsSolid {
		return false, nil
	}

	if !tangle.checkTransactionOutputs(availableBalances, tx.Outputs()) {
		return false, fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.Id())
	}

	return true, nil
}

func (tangle *Tangle) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result map[transaction.OutputId]*transaction.CachedOutput) {
	result = make(map[transaction.OutputId]*transaction.CachedOutput)
	tx.Inputs().ForEach(func(inputId transaction.OutputId) bool {
		result[inputId] = tangle.GetTransactionOutput(inputId)

		return true
	})

	return
}

// checkTransactionOutputs is a utility function that returns true, if the outputs are consuming all of the given inputs
// (the sum of all the balance changes is 0). It also accounts for the ability to "recolor" coins during the creating of
// outputs. If this function returns false, then the outputs that are defined in the transaction are invalid and the
// transaction should be removed from the ledger state.
func (tangle *Tangle) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
	// create a variable to keep track of outputs that create a new color
	var newlyColoredCoins int64

	// iterate through outputs and check them one by one
	aborted := !outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		for _, outputBalance := range balances {
			// abort if the output creates a negative or empty output
			if outputBalance.Value() <= 0 {
				return false
			}

			// sidestep logic if we have a newly colored output (we check the supply later)
			if outputBalance.Color() == balance.COLOR_New {
				// catch overflows
				if newlyColoredCoins > math.MaxInt64-outputBalance.Value() {
					return false
				}

				newlyColoredCoins += outputBalance.Value()

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

			// subtract the spent coins from the supply of this transaction
			inputBalances[outputBalance.Color()] -= outputBalance.Value()

			// cleanup the entry in the supply map if we have exhausted all funds
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

	// the outputs are valid if they spend all outputs
	return unspentCoins == newlyColoredCoins
}

// isPayloadSolid returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) isPayloadSolid(payload *payload.Payload, metadata *PayloadMetadata) bool {
	if payload == nil || payload.IsDeleted() {
		return false
	}

	if metadata == nil || metadata.IsDeleted() {
		return false
	}

	if metadata.IsSolid() {
		return true
	}

	return tangle.isPayloadMarkedAsSolid(payload.TrunkId()) && tangle.isPayloadMarkedAsSolid(payload.BranchId())
}

// isPayloadMarkedAsSolid returns true if the payload was marked as solid already (by setting the corresponding flags
// in its metadata.
func (tangle *Tangle) isPayloadMarkedAsSolid(payloadId payload.Id) bool {
	if payloadId == payload.GenesisId {
		return true
	}

	transactionMetadataCached := tangle.GetPayloadMetadata(payloadId)
	if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
		transactionMetadataCached.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(NewMissingPayload(payloadId)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*MissingPayload).GetId())
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
