package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
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
		approverStorage:        osFactory.New(PrefixApprovers, ApproverFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:  osFactory.New(PrefixMissingMessage, MissingMessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: newMessageStoreEvents(),
	}

	return
}

// StoreMessage stores a new message to the message store.
func (m *MessageStore) StoreMessage(msg *Message) {
	// store message
	var cachedMessage *CachedMessage
	_tmp, msgIsNew := m.messageStorage.StoreIfAbsent(msg)
	if !msgIsNew {
		return
	}
	cachedMessage = &CachedMessage{CachedObject: _tmp}

	// store message metadata
	messageID := msg.ID()
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: m.messageMetadataStorage.Store(NewMessageMetadata(messageID))}

	// TODO: approval switch: we probably need to introduce approver types
	// store approvers
	msg.ForEachStrongParent(func(parent MessageID) {
		m.approverStorage.Store(NewApprover(parent, messageID)).Release()
	})

	// trigger events
	if m.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		m.Events.MissingMessageReceived.Trigger(&CachedMessageEvent{
			Message:         cachedMessage,
			MessageMetadata: cachedMsgMetadata})
	}

	// messages are stored, trigger MessageStored event to move on next check
	m.Events.MessageStored.Trigger(&CachedMessageEvent{
		Message:         cachedMessage,
		MessageMetadata: cachedMsgMetadata})

	cachedMessage.Release()
	cachedMsgMetadata.Release()
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

// Approvers retrieves the approvers of a message from the storage.
func (m *MessageStore) Approvers(messageID MessageID) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	m.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, messageID[:])
	return approvers
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

		// TODO: reconsider behavior with approval switch
		currentMsg.ForEachParent(func(parent Parent) {
			m.deleteApprover(parent.ID, messageID)
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

// deleteApprover deletes a message from the approverStorage.
func (m *MessageStore) deleteApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	idToDelete := make([]byte, MessageIDLength+MessageIDLength)
	copy(idToDelete[:MessageIDLength], approvedMessageID[:])
	copy(idToDelete[MessageIDLength:], approvingMessage[:])
	m.approverStorage.Delete(idToDelete)
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
