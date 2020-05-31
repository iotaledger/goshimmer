package tangle

import (
	"container/list"
	"time"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// MaxMissingTimeBeforeCleanup is  the max. amount of time a message can be marked as missing
	// before it is ultimately un-marked as missing.
	MaxMissingTimeBeforeCleanup = 30 * time.Second
	// MissingCheckInterval is the interval on which it is checked whether a missing
	// message is still missing.
	MissingCheckInterval = 5 * time.Second
)

// Tangle represents the base layer of messages.
type Tangle struct {
	messageStorage         *objectstorage.ObjectStorage
	messageMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingMessageStorage  *objectstorage.ObjectStorage

	Events Events

	storeMessageWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	shutdown               chan struct{}
}

func messageFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return message.StorableObjectFromKey(key)
}

func approverFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ApproverFromStorageKey(key)
}

func missingMessageFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return MissingMessageFromStorageKey(key)
}

// New creates a new Tangle.
func New(store kvstore.KVStore) (result *Tangle) {
	osFactory := objectstorage.NewFactory(store, storageprefix.MessageLayer)

	result = &Tangle{
		shutdown:               make(chan struct{}),
		messageStorage:         osFactory.New(PrefixMessage, messageFactory, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage: osFactory.New(PrefixMessageMetadata, MessageMetadataFromStorageKey, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:        osFactory.New(PrefixApprovers, approverFactory, objectstorage.CacheTime(10*time.Second), objectstorage.PartitionKey(message.IdLength, message.IdLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, missingMessageFactory, objectstorage.CacheTime(10*time.Second), objectstorage.LeakDetectionEnabled(false)),

		Events: *newEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)
	return
}

// AttachMessage attaches a new message to the tangle.
func (tangle *Tangle) AttachMessage(msg *message.Message) {
	tangle.storeMessageWorkerPool.Submit(func() { tangle.storeMessageWorker(msg) })
}

// Message retrieves a message from the tangle.
func (tangle *Tangle) Message(messageId message.Id) *message.CachedMessage {
	return &message.CachedMessage{CachedObject: tangle.messageStorage.Load(messageId[:])}
}

// MessageMetadata retrieves the metadata of a message from the tangle.
func (tangle *Tangle) MessageMetadata(messageId message.Id) *CachedMessageMetadata {
	return &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Load(messageId[:])}
}

// Approvers retrieves the approvers of a message from the tangle.
func (tangle *Tangle) Approvers(messageId message.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, messageId[:])
	return approvers
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (tangle *Tangle) DeleteMessage(messageId message.Id) {
	tangle.Message(messageId).Consume(func(currentMsg *message.Message) {
		trunkMsgId := currentMsg.TrunkId()
		tangle.deleteApprover(trunkMsgId, messageId)

		branchMsgId := currentMsg.BranchId()
		if branchMsgId != trunkMsgId {
			tangle.deleteApprover(branchMsgId, messageId)
		}

		tangle.messageMetadataStorage.Delete(messageId[:])
		tangle.messageStorage.Delete(messageId[:])

		tangle.Events.MessageRemoved.Trigger(messageId)
	})
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storeMessageWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()

	tangle.messageStorage.Shutdown()
	tangle.messageMetadataStorage.Shutdown()
	tangle.approverStorage.Shutdown()
	tangle.missingMessageStorage.Shutdown()
	close(tangle.shutdown)

	return tangle
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
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

// worker that stores the message and calls the corresponding storage events.
func (tangle *Tangle) storeMessageWorker(msg *message.Message) {
	// store message
	var cachedMessage *message.CachedMessage
	_tmp, msgIsNew := tangle.messageStorage.StoreIfAbsent(msg)
	if !msgIsNew {
		return
	}
	cachedMessage = &message.CachedMessage{CachedObject: _tmp}

	// store message metadata
	messageId := msg.Id()
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Store(NewMessageMetadata(messageId))}

	// store trunk approver
	trunkMsgId := msg.TrunkId()
	tangle.approverStorage.Store(NewApprover(trunkMsgId, messageId)).Release()

	// store branch approver
	if branchMsgId := msg.BranchId(); branchMsgId != trunkMsgId {
		tangle.approverStorage.Store(NewApprover(branchMsgId, messageId)).Release()
	}

	// trigger events
	if tangle.missingMessageStorage.DeleteIfPresent(messageId[:]) {
		tangle.Events.MissingMessageReceived.Trigger(cachedMessage, cachedMsgMetadata)
	}
	tangle.Events.MessageAttached.Trigger(cachedMessage, cachedMsgMetadata)

	// check message solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.checkMessageSolidityAndPropagate(cachedMessage, cachedMsgMetadata)
	})
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (tangle *Tangle) isMessageMarkedAsSolid(messageId message.Id) bool {
	if messageId == message.EmptyId {
		return true
	}

	msgMetadataCached := tangle.MessageMetadata(messageId)
	defer msgMetadataCached.Release()
	msgMetadata := msgMetadataCached.Unwrap()

	// mark message as missing
	if msgMetadata == nil {
		missingMessage := NewMissingMessage(messageId)
		if cachedMissingMessage, stored := tangle.missingMessageStorage.StoreIfAbsent(missingMessage); stored {
			cachedMissingMessage.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.MessageMissing.Trigger(messageId)
			})
		}
		return false
	}

	return msgMetadata.IsSolid()
}

// checks whether the given message is solid by examining whether its trunk and
// branch messages are solid.
func (tangle *Tangle) isMessageSolid(msg *message.Message, msgMetadata *MessageMetadata) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	if msgMetadata == nil || msgMetadata.IsDeleted() {
		return false
	}

	if msgMetadata.IsSolid() {
		return true
	}

	return tangle.isMessageMarkedAsSolid(msg.TrunkId()) && tangle.isMessageMarkedAsSolid(msg.BranchId())
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (tangle *Tangle) checkMessageSolidityAndPropagate(cachedMessage *message.CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

	popElementsFromStack := func(stack *list.List) (*message.CachedMessage, *CachedMessageMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedMsg := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedMsgMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)
		return currentCachedMsg.(*message.CachedMessage), currentCachedMsgMetadata.(*CachedMessageMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedMessage, cachedMsgMetadata})

	// processed messages that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedMessage, currentCachedMsgMetadata := popElementsFromStack(solidificationStack)

		currentMessage := currentCachedMessage.Unwrap()
		currentMsgMetadata := currentCachedMsgMetadata.Unwrap()
		if currentMessage == nil || currentMsgMetadata == nil {
			currentCachedMessage.Release()
			currentCachedMsgMetadata.Release()
			continue
		}

		// mark the message as solid if it has become solid
		if tangle.isMessageSolid(currentMessage, currentMsgMetadata) && currentMsgMetadata.SetSolid(true) {
			tangle.Events.MessageSolid.Trigger(currentCachedMessage, currentCachedMsgMetadata)

			// auto. push approvers of the newly solid message to propagate solidification
			tangle.Approvers(currentMessage.Id()).Consume(func(approver *Approver) {
				approverMessageId := approver.ApproverMessageId()
				solidificationStack.PushBack([2]interface{}{
					tangle.Message(approverMessageId),
					tangle.MessageMetadata(approverMessageId),
				})
			})
		}

		currentCachedMessage.Release()
		currentCachedMsgMetadata.Release()
	}
}

// MonitorMissingMessages continuously monitors for missing messages and eventually deletes them if they
// don't become available in a certain time frame.
func (tangle *Tangle) MonitorMissingMessages(shutdownSignal <-chan struct{}) {
	reCheckInterval := time.NewTicker(MissingCheckInterval)
	defer reCheckInterval.Stop()
	for {
		select {
		case <-reCheckInterval.C:
			var toDelete []message.Id
			tangle.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
				defer cachedObject.Release()
				missingMessage := cachedObject.Get().(*MissingMessage)

				// check whether message is missing since over our max time delta
				if time.Since(missingMessage.MissingSince()) >= MaxMissingTimeBeforeCleanup {
					toDelete = append(toDelete, missingMessage.MessageId())
				}
				return true
			})
			for _, msgID := range toDelete {
				// delete the future cone of the missing message
				tangle.Events.MessageUnsolidifiable.Trigger(msgID)
				// TODO: obvious race condition between receiving the message and it getting deleted here
				tangle.deleteFutureCone(msgID)
			}
		case <-shutdownSignal:
			return
		}
	}
}

// deletes the given approver association for the given approvee to its approver.
func (tangle *Tangle) deleteApprover(approvedMessageId message.Id, approvingMessage message.Id) {
	idToDelete := make([]byte, message.IdLength+message.IdLength)
	copy(idToDelete[:message.IdLength], approvedMessageId[:])
	copy(idToDelete[message.IdLength:], approvingMessage[:])
	tangle.approverStorage.Delete(idToDelete)
}

// deletes a message and its future cone of messages/approvers.
func (tangle *Tangle) deleteFutureCone(messageId message.Id) {
	cleanupStack := list.New()
	cleanupStack.PushBack(messageId)

	processedMessages := make(map[message.Id]types.Empty)
	processedMessages[messageId] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentMessageId := currentStackEntry.Value.(message.Id)
		cleanupStack.Remove(currentStackEntry)

		tangle.DeleteMessage(currentMessageId)

		tangle.Approvers(currentMessageId).Consume(func(approver *Approver) {
			approverId := approver.ApproverMessageId()
			if _, messageProcessed := processedMessages[approverId]; !messageProcessed {
				cleanupStack.PushBack(approverId)
				processedMessages[approverId] = types.Void
			}
		})
	}
}
