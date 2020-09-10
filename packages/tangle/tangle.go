package tangle

import (
	"container/list"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
)

const (
	cacheTime = 20 * time.Second
)

// Tangle represents the base layer of messages.
type Tangle struct {
	messageStorage         *objectstorage.ObjectStorage
	messageMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingMessageStorage  *objectstorage.ObjectStorage

	Events *TangleEvents

	storeMessageWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	shutdown               chan struct{}
}

func messageFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return StorableObjectFromKey(key)
}

func approverFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ApproverFromStorageKey(key)
}

func missingMessageFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return MissingMessageFromStorageKey(key)
}

// NewTangle creates a new Tangle.
func NewTangle(store kvstore.KVStore) (result *Tangle) {
	osFactory := objectstorage.NewFactory(store, storageprefix.MessageLayer)

	result = &Tangle{
		shutdown:               make(chan struct{}),
		messageStorage:         osFactory.New(PrefixMessage, messageFactory, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage: osFactory.New(PrefixMessageMetadata, MessageMetadataFromStorageKey, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:        osFactory.New(PrefixApprovers, approverFactory, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, missingMessageFactory, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: newTangleEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)
	result.storeMessageWorkerPool.Tune(1024)
	return
}

// AttachMessage attaches a new message to the tangle.
func (tangle *Tangle) AttachMessage(msg *Message) {
	tangle.storeMessageWorkerPool.Submit(func() { tangle.storeMessageWorker(msg) })
}

// Message retrieves a message from the tangle.
func (tangle *Tangle) Message(messageID MessageID) *CachedMessage {
	return &CachedMessage{CachedObject: tangle.messageStorage.Load(messageID[:])}
}

// MessageMetadata retrieves the metadata of a message from the tangle.
func (tangle *Tangle) MessageMetadata(messageID MessageID) *CachedMessageMetadata {
	return &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Load(messageID[:])}
}

// Approvers retrieves the approvers of a message from the tangle.
func (tangle *Tangle) Approvers(messageID MessageID) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, messageID[:])
	return approvers
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (tangle *Tangle) DeleteMessage(messageID MessageID) {
	tangle.Message(messageID).Consume(func(currentMsg *Message) {
		parent1MsgID := currentMsg.Parent1ID()
		tangle.deleteApprover(parent1MsgID, messageID)

		parent2MsgID := currentMsg.Parent2ID()
		if parent2MsgID != parent1MsgID {
			tangle.deleteApprover(parent2MsgID, messageID)
		}

		tangle.messageMetadataStorage.Delete(messageID[:])
		tangle.messageStorage.Delete(messageID[:])

		tangle.Events.MessageRemoved.Trigger(messageID)
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (tangle *Tangle) DeleteMissingMessage(messageID MessageID) {
	tangle.missingMessageStorage.Delete(messageID[:])
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

// DBStats returns the number of solid messages and total number of messages in the database (messageMetadataStorage,
// that should contain the messages as messageStorage), the number of messages in missingMessageStorage, furthermore
// the average time it takes to solidify messages.
func (tangle *Tangle) DBStats() (solidCount int, messageCount int, avgSolidificationTime float64, missingMessageCount int) {
	var sumSolidificationTime time.Duration
	tangle.messageMetadataStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			msgMetaData := object.(*MessageMetadata)
			messageCount++
			received := msgMetaData.ReceivedTime()
			if msgMetaData.IsSolid() {
				solidCount++
				sumSolidificationTime += msgMetaData.solidificationTime.Sub(received)
			}
		})
		return true
	})
	if solidCount > 0 {
		avgSolidificationTime = float64(sumSolidificationTime.Milliseconds()) / float64(solidCount)
	}
	tangle.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			missingMessageCount++
		})
		return true
	})
	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (tangle *Tangle) MissingMessages() (ids []MessageID) {
	tangle.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			ids = append(ids, object.(*MissingMessage).messageID)
		})

		return true
	})
	return
}

// worker that stores the message and calls the corresponding storage events.
func (tangle *Tangle) storeMessageWorker(msg *Message) {
	// store message
	var cachedMessage *CachedMessage
	_tmp, msgIsNew := tangle.messageStorage.StoreIfAbsent(msg)
	if !msgIsNew {
		return
	}
	cachedMessage = &CachedMessage{CachedObject: _tmp}

	// store message metadata
	messageID := msg.ID()
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: tangle.messageMetadataStorage.Store(NewMessageMetadata(messageID))}

	// store parent1 approver
	parent1MsgID := msg.Parent1ID()
	tangle.approverStorage.Store(NewApprover(parent1MsgID, messageID)).Release()

	// store parent2 approver
	if parent2MsgID := msg.Parent2ID(); parent2MsgID != parent1MsgID {
		tangle.approverStorage.Store(NewApprover(parent2MsgID, messageID)).Release()
	}

	// trigger events
	if tangle.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		tangle.Events.MissingMessageReceived.Trigger(&CachedMessageEvent{
			Message:         cachedMessage,
			MessageMetadata: cachedMsgMetadata})
	}

	tangle.Events.MessageAttached.Trigger(&CachedMessageEvent{
		Message:         cachedMessage,
		MessageMetadata: cachedMsgMetadata})

	// check message solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.checkMessageSolidityAndPropagate(cachedMessage, cachedMsgMetadata)
	})
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (tangle *Tangle) isMessageMarkedAsSolid(messageID MessageID) bool {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata and mark it as missing if it doesn't exist
	msgMetadataCached := &CachedMessageMetadata{tangle.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) objectstorage.StorableObject {
		// store the missing message and trigger events
		if cachedMissingMessage, stored := tangle.missingMessageStorage.StoreIfAbsent(NewMissingMessage(messageID)); stored {
			cachedMissingMessage.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.MessageMissing.Trigger(messageID)
			})
		}

		// do not initialize the metadata here, we execute this in ComputeIfAbsent to be secure from race conditions
		return nil
	})}
	defer msgMetadataCached.Release()

	// return false if the metadata does not exist
	msgMetadata := msgMetadataCached.Unwrap()
	if msgMetadata == nil {
		return false
	}

	// return the solid flag of the metadata object
	return msgMetadata.IsSolid()
}

// checks whether the given message is solid by examining whether its parent1 and
// parent2 messages are solid.
func (tangle *Tangle) isMessageSolid(msg *Message, msgMetadata *MessageMetadata) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	if msgMetadata == nil || msgMetadata.IsDeleted() {
		return false
	}

	if msgMetadata.IsSolid() {
		return true
	}

	// as missing messages are requested in isMessageMarkedAsSolid, we want to prevent short-circuit evaluation
	parent1Solid := tangle.isMessageMarkedAsSolid(msg.Parent1ID())
	parent2Solid := tangle.isMessageMarkedAsSolid(msg.Parent2ID())
	return parent1Solid && parent2Solid
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (tangle *Tangle) checkMessageSolidityAndPropagate(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

	popElementsFromStack := func(stack *list.List) (*CachedMessage, *CachedMessageMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedMsg := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedMsgMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)
		return currentCachedMsg.(*CachedMessage), currentCachedMsgMetadata.(*CachedMessageMetadata)
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
			tangle.Events.MessageSolid.Trigger(&CachedMessageEvent{
				Message:         currentCachedMessage,
				MessageMetadata: currentCachedMsgMetadata})

			// auto. push approvers of the newly solid message to propagate solidification
			tangle.Approvers(currentMessage.ID()).Consume(func(approver *Approver) {
				approverMessageID := approver.ApproverMessageID()
				solidificationStack.PushBack([2]interface{}{
					tangle.Message(approverMessageID),
					tangle.MessageMetadata(approverMessageID),
				})
			})
		}

		currentCachedMessage.Release()
		currentCachedMsgMetadata.Release()
	}
}

// deletes the given approver association for the given approvee to its approver.
func (tangle *Tangle) deleteApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	idToDelete := make([]byte, MessageIDLength+MessageIDLength)
	copy(idToDelete[:MessageIDLength], approvedMessageID[:])
	copy(idToDelete[MessageIDLength:], approvingMessage[:])
	tangle.approverStorage.Delete(idToDelete)
}

// deletes a message and its future cone of messages/approvers.
// nolint
func (tangle *Tangle) deleteFutureCone(messageID MessageID) {
	cleanupStack := list.New()
	cleanupStack.PushBack(messageID)

	processedMessages := make(map[MessageID]types.Empty)
	processedMessages[messageID] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentMessageID := currentStackEntry.Value.(MessageID)
		cleanupStack.Remove(currentStackEntry)

		tangle.DeleteMessage(currentMessageID)

		tangle.Approvers(currentMessageID).Consume(func(approver *Approver) {
			approverID := approver.ApproverMessageID()
			if _, messageProcessed := processedMessages[approverID]; !messageProcessed {
				cleanupStack.PushBack(approverID)
				processedMessages[approverID] = types.Void
			}
		})
	}
}

// SolidifierWorkerPoolStatus returns the name and the load of the workerpool.
func (tangle *Tangle) SolidifierWorkerPoolStatus() (name string, load int) {
	return "Solidifier", tangle.solidifierWorkerPool.RunningWorkers()
}

// StoreMessageWorkerPoolStatus returns the name and the load of the workerpool.
func (tangle *Tangle) StoreMessageWorkerPoolStatus() (name string, load int) {
	return "StoreMessage", tangle.storeMessageWorkerPool.RunningWorkers()
}

// RetrieveAllTips returns the tips (i.e., solid messages that are not part of the approvers list).
// It iterates over the messageMetadataStorage, thus only use this method if necessary.
// TODO: improve this function.
func (tangle *Tangle) RetrieveAllTips() (tips []MessageID) {
	tangle.messageMetadataStorage.ForEach(func(key []byte, cachedMessage objectstorage.CachedObject) bool {
		cachedMessage.Consume(func(object objectstorage.StorableObject) {
			messageMetadata := object.(*MessageMetadata)
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := tangle.Approvers(messageMetadata.messageID)
				if len(cachedApprovers) == 0 {
					tips = append(tips, messageMetadata.messageID)
				}
				cachedApprovers.Consume(func(approver *Approver) {})
			}
		})
		return true
	})
	return tips
}
