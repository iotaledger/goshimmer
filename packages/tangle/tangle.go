package tangle

import (
	"container/list"
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
)

const (
	// PrefixMessage defines the storage prefix for message.
	PrefixMessage byte = iota
	// PrefixMessageMetadata defines the storage prefix for message metadata.
	PrefixMessageMetadata
	// PrefixApprovers defines the storage prefix for approvers.
	PrefixApprovers
	// PrefixMissingMessage defines the storage prefix for missing message.
	PrefixMissingMessage

	cacheTime = 20 * time.Second

	// DBSequenceNumber defines the db sequence number.
	DBSequenceNumber = "seq"
)

// Tangle represents the base layer of messages.
type Tangle struct {
	messageStorage         *objectstorage.ObjectStorage
	messageMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingMessageStorage  *objectstorage.ObjectStorage

	Events *Events

	storeMessageWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	shutdown               chan struct{}
}

// New creates a new Tangle.
func New(store kvstore.KVStore) (result *Tangle) {
	osFactory := objectstorage.NewFactory(store, database.PrefixMessageLayer)

	result = &Tangle{
		shutdown:               make(chan struct{}),
		messageStorage:         osFactory.New(PrefixMessage, MessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage: osFactory.New(PrefixMessageMetadata, MessageMetadataFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:        osFactory.New(PrefixApprovers, ApproverFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, MissingMessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: newEvents(),
	}

	result.solidifierWorkerPool.Tune(1024)
	result.storeMessageWorkerPool.Tune(1024)
	return
}

// AttachMessage attaches a new message to the tangle.
func (t *Tangle) AttachMessage(msg *Message) {
	t.storeMessageWorkerPool.Submit(func() { t.storeMessageWorker(msg) })
}

// Message retrieves a message from the tangle.
func (t *Tangle) Message(messageID MessageID) *CachedMessage {
	return &CachedMessage{CachedObject: t.messageStorage.Load(messageID[:])}
}

// MessageMetadata retrieves the metadata of a message from the tangle.
func (t *Tangle) MessageMetadata(messageID MessageID) *CachedMessageMetadata {
	return &CachedMessageMetadata{CachedObject: t.messageMetadataStorage.Load(messageID[:])}
}

// Approvers retrieves the approvers of a message from the tangle.
func (t *Tangle) Approvers(messageID MessageID) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	t.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, messageID[:])
	return approvers
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (t *Tangle) DeleteMessage(messageID MessageID) {
	t.Message(messageID).Consume(func(currentMsg *Message) {

		// TODO: reconsider behavior with approval switch
		currentMsg.ForEachParent(func(parent Parent) {
			t.deleteApprover(parent.ID, messageID)
		})

		t.messageMetadataStorage.Delete(messageID[:])
		t.messageStorage.Delete(messageID[:])

		t.Events.MessageRemoved.Trigger(messageID)
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (t *Tangle) DeleteMissingMessage(messageID MessageID) {
	t.missingMessageStorage.Delete(messageID[:])
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() *Tangle {
	t.storeMessageWorkerPool.ShutdownGracefully()
	t.solidifierWorkerPool.ShutdownGracefully()

	t.messageStorage.Shutdown()
	t.messageMetadataStorage.Shutdown()
	t.approverStorage.Shutdown()
	t.missingMessageStorage.Shutdown()
	close(t.shutdown)

	return t
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (t *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		t.messageStorage,
		t.messageMetadataStorage,
		t.approverStorage,
		t.missingMessageStorage,
	} {
		if err := storage.Prune(); err != nil {
			err = fmt.Errorf("failed to prune storage: %w", err)
			return err
		}
	}

	return nil
}

// DBStats returns the number of solid messages and total number of messages in the database (messageMetadataStorage,
// that should contain the messages as messageStorage), the number of messages in missingMessageStorage, furthermore
// the average time it takes to solidify messages.
func (t *Tangle) DBStats() (solidCount int, messageCount int, avgSolidificationTime float64, missingMessageCount int) {
	var sumSolidificationTime time.Duration
	t.messageMetadataStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
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
	t.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			missingMessageCount++
		})
		return true
	})
	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (t *Tangle) MissingMessages() (ids []MessageID) {
	t.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			ids = append(ids, object.(*MissingMessage).messageID)
		})

		return true
	})
	return
}

// worker that stores the message and calls the corresponding storage events.
func (t *Tangle) storeMessageWorker(msg *Message) {
	// store message
	var cachedMessage *CachedMessage
	_tmp, msgIsNew := t.messageStorage.StoreIfAbsent(msg)
	if !msgIsNew {
		return
	}
	cachedMessage = &CachedMessage{CachedObject: _tmp}

	// store message metadata
	messageID := msg.ID()
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: t.messageMetadataStorage.Store(NewMessageMetadata(messageID))}

	// TODO: approval switch: we probably need to introduce approver types
	// store approvers
	msg.ForEachStrongParent(func(parent MessageID) {
		t.approverStorage.Store(NewApprover(parent, messageID)).Release()
	})

	// trigger events
	if t.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		t.Events.MissingMessageReceived.Trigger(&CachedMessageEvent{
			Message:         cachedMessage,
			MessageMetadata: cachedMsgMetadata})
	}

	t.Events.MessageAttached.Trigger(&CachedMessageEvent{
		Message:         cachedMessage,
		MessageMetadata: cachedMsgMetadata})

	// check message solidity
	t.solidifierWorkerPool.Submit(func() {
		t.checkMessageSolidityAndPropagate(cachedMessage, cachedMsgMetadata)
	})
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (t *Tangle) isMessageMarkedAsSolid(messageID MessageID) bool {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata and mark it as missing if it doesn't exist
	msgMetadataCached := &CachedMessageMetadata{t.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) objectstorage.StorableObject {
		// store the missing message and trigger events
		if cachedMissingMessage, stored := t.missingMessageStorage.StoreIfAbsent(NewMissingMessage(messageID)); stored {
			cachedMissingMessage.Consume(func(object objectstorage.StorableObject) {
				t.Events.MessageMissing.Trigger(messageID)
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
func (t *Tangle) isMessageSolid(msg *Message, msgMetadata *MessageMetadata) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	if msgMetadata == nil || msgMetadata.IsDeleted() {
		return false
	}

	if msgMetadata.IsSolid() {
		return true
	}

	solid := true

	msg.ForEachParent(func(parent Parent) {
		// as missing messages are requested in isMessageMarkedAsSolid,
		// we want to prevent short-circuit evaluation, thus we need to use a tmp variable
		// to avoid side effects from comparing directly to the function call.
		tmp := t.isMessageMarkedAsSolid(parent.ID)
		solid = solid && tmp
	})

	return solid
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (t *Tangle) checkMessageSolidityAndPropagate(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

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
		if t.isMessageSolid(currentMessage, currentMsgMetadata) && currentMsgMetadata.SetSolid(true) {
			t.Events.MessageSolid.Trigger(&CachedMessageEvent{
				Message:         currentCachedMessage,
				MessageMetadata: currentCachedMsgMetadata})

			// auto. push approvers of the newly solid message to propagate solidification
			t.Approvers(currentMessage.ID()).Consume(func(approver *Approver) {
				approverMessageID := approver.ApproverMessageID()
				solidificationStack.PushBack([2]interface{}{
					t.Message(approverMessageID),
					t.MessageMetadata(approverMessageID),
				})
			})
		}

		currentCachedMessage.Release()
		currentCachedMsgMetadata.Release()
	}
}

// deletes the given approver association for the given approvee to its approver.
func (t *Tangle) deleteApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	idToDelete := make([]byte, MessageIDLength+MessageIDLength)
	copy(idToDelete[:MessageIDLength], approvedMessageID[:])
	copy(idToDelete[MessageIDLength:], approvingMessage[:])
	t.approverStorage.Delete(idToDelete)
}

// deletes a message and its future cone of messages/approvers.
// nolint
func (t *Tangle) deleteFutureCone(messageID MessageID) {
	cleanupStack := list.New()
	cleanupStack.PushBack(messageID)

	processedMessages := make(map[MessageID]types.Empty)
	processedMessages[messageID] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentMessageID := currentStackEntry.Value.(MessageID)
		cleanupStack.Remove(currentStackEntry)

		t.DeleteMessage(currentMessageID)

		t.Approvers(currentMessageID).Consume(func(approver *Approver) {
			approverID := approver.ApproverMessageID()
			if _, messageProcessed := processedMessages[approverID]; !messageProcessed {
				cleanupStack.PushBack(approverID)
				processedMessages[approverID] = types.Void
			}
		})
	}
}

// SolidifierWorkerPoolStatus returns the name and the load of the workerpool.
func (t *Tangle) SolidifierWorkerPoolStatus() (name string, load int) {
	return "Solidifier", t.solidifierWorkerPool.RunningWorkers()
}

// StoreMessageWorkerPoolStatus returns the name and the load of the workerpool.
func (t *Tangle) StoreMessageWorkerPoolStatus() (name string, load int) {
	return "StoreMessage", t.storeMessageWorkerPool.RunningWorkers()
}

// RetrieveAllTips returns the tips (i.e., solid messages that are not part of the approvers list).
// It iterates over the messageMetadataStorage, thus only use this method if necessary.
// TODO: improve this function.
func (t *Tangle) RetrieveAllTips() (tips []MessageID) {
	t.messageMetadataStorage.ForEach(func(key []byte, cachedMessage objectstorage.CachedObject) bool {
		cachedMessage.Consume(func(object objectstorage.StorableObject) {
			messageMetadata := object.(*MessageMetadata)
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := t.Approvers(messageMetadata.messageID)
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
