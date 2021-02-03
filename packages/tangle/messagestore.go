package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
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

// MessageStore represents the storage of messages.
type MessageStore struct {
	messageStorage         *objectstorage.ObjectStorage
	messageMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingMessageStorage  *objectstorage.ObjectStorage

	Events   *MessageStoreEvents
	shutdown chan struct{}
}

// NewMessageStore creates a new MessageStore.
func NewMessageStore(store kvstore.KVStore) (result *MessageStore) {
	osFactory := objectstorage.NewFactory(store, database.PrefixMessageLayer)

	result = &MessageStore{
		shutdown:               make(chan struct{}),
		messageStorage:         osFactory.New(PrefixMessage, MessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage: osFactory.New(PrefixMessageMetadata, MessageMetadataFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:        osFactory.New(PrefixApprovers, ApproverFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, MissingMessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: newMessageStoreEvents(),
	}

	return
}

// StoreMessage stores a new message to the message store.
func (m *MessageStore) StoreMessage(msg *Message) {
	// retrieve MessageID
	messageID := msg.ID()

	// store Messages only once by using the existence of the Metadata as a guard
	storedMetadata, stored := m.messageMetadataStorage.StoreIfAbsent(NewMessageMetadata(messageID))
	if !stored {
		return
	}

	// create typed version of the stored MessageMetadata
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: storedMetadata}
	defer cachedMsgMetadata.Release()

	// store Message
	cachedMessage := &CachedMessage{CachedObject: m.messageStorage.Store(msg)}
	defer cachedMessage.Release()

	// TODO: approval switch: we probably need to introduce approver types
	// store approvers
	msg.ForEachStrongParent(func(parentMessageID MessageID) {
		m.approverStorage.Store(NewApprover(StrongApprover, parentMessageID, messageID)).Release()
	})
	msg.ForEachWeakParent(func(parentMessageID MessageID) {
		m.approverStorage.Store(NewApprover(WeakApprover, parentMessageID, messageID)).Release()
	})

	// trigger events
	if m.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		m.Events.MissingMessageReceived.Trigger(&CachedMessageEvent{
			Message:         cachedMessage,
			MessageMetadata: cachedMsgMetadata,
		})
	}

	// messages are stored, trigger MessageStored event to move on next check
	m.Events.MessageStored.Trigger(&CachedMessageEvent{
		Message:         cachedMessage,
		MessageMetadata: cachedMsgMetadata,
	})
}

// Message retrieves a message from the message store.
func (m *MessageStore) Message(messageID MessageID) *CachedMessage {
	return &CachedMessage{CachedObject: m.messageStorage.Load(messageID[:])}
}

// MessageMetadata retrieves the metadata of a message from the storage.
func (m *MessageStore) MessageMetadata(messageID MessageID) *CachedMessageMetadata {
	return &CachedMessageMetadata{CachedObject: m.messageMetadataStorage.Load(messageID[:])}
}

// StoreIfMissingMessageMetadata retrieves the CachedMessageMetadata and mark it as missing if it doesn't exist
func (m *MessageStore) StoreIfMissingMessageMetadata(messageID MessageID) *CachedMessageMetadata {
	return &CachedMessageMetadata{m.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) objectstorage.StorableObject {
		// store the missing message and trigger events
		if cachedMissingMessage, stored := m.missingMessageStorage.StoreIfAbsent(NewMissingMessage(messageID)); stored {
			cachedMissingMessage.Consume(func(object objectstorage.StorableObject) {
				m.Events.MessageMissing.Trigger(messageID)
			})
		}

		// do not initialize the metadata here, we execute this in ComputeIfAbsent to be secure from race conditions
		return nil
	})}
}

// Approvers retrieves the Approvers of a Message from the object storage. It is possible to provide an optional
// ApproverType to only return the corresponding Approvers.
func (m *MessageStore) Approvers(messageID MessageID, optionalApproverType ...ApproverType) (cachedApprovers CachedApprovers) {
	var iterationPrefix []byte
	if len(optionalApproverType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(messageID.Bytes(), optionalApproverType[0].Bytes())
	} else {
		iterationPrefix = messageID.Bytes()
	}

	cachedApprovers = make(CachedApprovers, 0)
	m.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedApprovers = append(cachedApprovers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, iterationPrefix)

	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (m *MessageStore) MissingMessages() (ids []MessageID) {
	m.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			ids = append(ids, object.(*MissingMessage).messageID)
		})

		return true
	})
	return
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (m *MessageStore) DeleteMessage(messageID MessageID) {
	m.Message(messageID).Consume(func(currentMsg *Message) {
		currentMsg.ForEachStrongParent(func(parentMessageID MessageID) {
			m.deleteStrongApprover(parentMessageID, messageID)
		})
		currentMsg.ForEachWeakParent(func(parentMessageID MessageID) {
			m.deleteWeakApprover(parentMessageID, messageID)
		})

		m.messageMetadataStorage.Delete(messageID[:])
		m.messageStorage.Delete(messageID[:])

		m.Events.MessageRemoved.Trigger(messageID)
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (m *MessageStore) DeleteMissingMessage(messageID MessageID) {
	m.missingMessageStorage.Delete(messageID[:])
}

// deleteStrongApprover deletes an Approver from the object storage that was created by a strong parent.
func (m *MessageStore) deleteStrongApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	m.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), StrongApprover.Bytes(), approvingMessage.Bytes()))
}

// deleteWeakApprover deletes an Approver from the object storage that was created by a weak parent.
func (m *MessageStore) deleteWeakApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	m.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), WeakApprover.Bytes(), approvingMessage.Bytes()))
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (m *MessageStore) Shutdown() {
	m.messageStorage.Shutdown()
	m.messageMetadataStorage.Shutdown()
	m.approverStorage.Shutdown()
	m.missingMessageStorage.Shutdown()

	close(m.shutdown)
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (m *MessageStore) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		m.messageStorage,
		m.messageMetadataStorage,
		m.approverStorage,
		m.missingMessageStorage,
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
func (m *MessageStore) DBStats() (solidCount int, messageCount int, avgSolidificationTime float64, missingMessageCount int) {
	var sumSolidificationTime time.Duration
	m.messageMetadataStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
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
	m.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			missingMessageCount++
		})
		return true
	})
	return
}

// RetrieveAllTips returns the tips (i.e., solid messages that are not part of the approvers list).
// It iterates over the messageMetadataStorage, thus only use this method if necessary.
// TODO: improve this function.
func (m *MessageStore) RetrieveAllTips() (tips []MessageID) {
	m.messageMetadataStorage.ForEach(func(key []byte, cachedMessage objectstorage.CachedObject) bool {
		cachedMessage.Consume(func(object objectstorage.StorableObject) {
			messageMetadata := object.(*MessageMetadata)
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := m.Approvers(messageMetadata.messageID)
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
