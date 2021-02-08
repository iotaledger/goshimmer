package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
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

	// PrefixAttachments defines the storage prefix for attachments.
	PrefixAttachments

	// PrefixMarkerBranchIDMapping defines the storage prefix for the PrefixMarkerBranchIDMapping.
	PrefixMarkerBranchIDMapping

	cacheTime = 20 * time.Second

	// DBSequenceNumber defines the db sequence number.
	DBSequenceNumber = "seq"
)

// Storage represents the storage of messages.
type Storage struct {
	messageStorage                    *objectstorage.ObjectStorage
	messageMetadataStorage            *objectstorage.ObjectStorage
	approverStorage                   *objectstorage.ObjectStorage
	missingMessageStorage             *objectstorage.ObjectStorage
	attachmentStorage                 *objectstorage.ObjectStorage
	markerIndexBranchIDMappingStorage *objectstorage.ObjectStorage

	Events   *MessageStoreEvents
	shutdown chan struct{}
}

// NewStorage creates a new Storage.
func NewStorage(store kvstore.KVStore) (result *Storage) {
	osFactory := objectstorage.NewFactory(store, database.PrefixMessageLayer)

	result = &Storage{
		shutdown:                          make(chan struct{}),
		messageStorage:                    osFactory.New(PrefixMessage, MessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage:            osFactory.New(PrefixMessageMetadata, MessageMetadataFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:                   osFactory.New(PrefixApprovers, ApproverFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:             osFactory.New(PrefixMissingMessage, MissingMessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		attachmentStorage:                 osFactory.New(PrefixAttachments, AttachmentFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(ledgerstate.TransactionIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		markerIndexBranchIDMappingStorage: osFactory.New(PrefixMarkerBranchIDMapping, MarkerIndexBranchIDMappingFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: newMessageStoreEvents(),
	}

	return
}

// StoreMessage stores a new message to the message store.
func (m *Storage) StoreMessage(message *Message) {
	// retrieve MessageID
	messageID := message.ID()

	// store Messages only once by using the existence of the Metadata as a guard
	storedMetadata, stored := m.messageMetadataStorage.StoreIfAbsent(NewMessageMetadata(messageID))
	if !stored {
		return
	}

	// create typed version of the stored MessageMetadata
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: storedMetadata}
	defer cachedMsgMetadata.Release()

	// store Message
	cachedMessage := &CachedMessage{CachedObject: m.messageStorage.Store(message)}
	defer cachedMessage.Release()

	// TODO: approval switch: we probably need to introduce approver types
	// store approvers
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		m.approverStorage.Store(NewApprover(StrongApprover, parentMessageID, messageID)).Release()
	})
	message.ForEachWeakParent(func(parentMessageID MessageID) {
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
	m.Events.MessageStored.Trigger(message.ID())
}

// Message retrieves a message from the message store.
func (m *Storage) Message(messageID MessageID) *CachedMessage {
	return &CachedMessage{CachedObject: m.messageStorage.Load(messageID[:])}
}

// MessageMetadata retrieves the MessageMetadata with the given MessageID.
func (m *Storage) MessageMetadata(messageID MessageID, optionalComputeIfAbsentCallback ...func() *MessageMetadata) *CachedMessageMetadata {
	if len(optionalComputeIfAbsentCallback) >= 1 {
		return &CachedMessageMetadata{m.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return optionalComputeIfAbsentCallback[0]()
		})}
	}

	return &CachedMessageMetadata{CachedObject: m.messageMetadataStorage.Load(messageID[:])}
}

// Approvers retrieves the Approvers of a Message from the object storage. It is possible to provide an optional
// ApproverType to only return the corresponding Approvers.
func (m *Storage) Approvers(messageID MessageID, optionalApproverType ...ApproverType) (cachedApprovers CachedApprovers) {
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

func (m *Storage) StoreMissingMessage(missingMessage *MissingMessage) (cachedMissingMessage *CachedMissingMessage, stored bool) {
	cachedObject, stored := m.missingMessageStorage.StoreIfAbsent(missingMessage)
	cachedMissingMessage = &CachedMissingMessage{CachedObject: cachedObject}

	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (m *Storage) MissingMessages() (ids []MessageID) {
	m.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			ids = append(ids, object.(*MissingMessage).messageID)
		})

		return true
	})
	return
}

// StoreAttachment stores a new attachment if not already stored.
func (m *Storage) StoreAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) (cachedAttachment *CachedAttachment, stored bool) {
	attachment, stored := m.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, messageID))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}
	return
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (m *Storage) Attachments(transactionID ledgerstate.TransactionID) (cachedAttachments CachedAttachments) {
	m.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedAttachments = append(cachedAttachments, &CachedAttachment{CachedObject: cachedObject})
		return true
	}, transactionID.Bytes())
	return
}

// AttachmentMessageIDs returns the messageIDs of the transaction in attachmentStorage.
func (m *Storage) AttachmentMessageIDs(transactionID ledgerstate.TransactionID) (messageIDs MessageIDs) {
	m.Attachments(transactionID).Consume(func(attachment *Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
	})
	return
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (m *Storage) DeleteMessage(messageID MessageID) {
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
func (m *Storage) DeleteMissingMessage(messageID MessageID) {
	m.missingMessageStorage.Delete(messageID[:])
}

func (m *Storage) MarkerIndexBranchIDMapping(sequenceID markers.SequenceID, optionalComputeIfAbsentCallback ...func(sequenceID markers.SequenceID) *MarkerIndexBranchIDMapping) *CachedMarkerIndexBranchIDMapping {
	if len(optionalComputeIfAbsentCallback) >= 1 {
		return &CachedMarkerIndexBranchIDMapping{m.messageMetadataStorage.ComputeIfAbsent(sequenceID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return optionalComputeIfAbsentCallback[0](sequenceID)
		})}
	}

	return &CachedMarkerIndexBranchIDMapping{CachedObject: m.messageMetadataStorage.Load(sequenceID.Bytes())}
}

// deleteStrongApprover deletes an Approver from the object storage that was created by a strong parent.
func (m *Storage) deleteStrongApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	m.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), StrongApprover.Bytes(), approvingMessage.Bytes()))
}

// deleteWeakApprover deletes an Approver from the object storage that was created by a weak parent.
func (m *Storage) deleteWeakApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	m.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), WeakApprover.Bytes(), approvingMessage.Bytes()))
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (m *Storage) Shutdown() {
	m.messageStorage.Shutdown()
	m.messageMetadataStorage.Shutdown()
	m.approverStorage.Shutdown()
	m.missingMessageStorage.Shutdown()
	m.attachmentStorage.Shutdown()

	close(m.shutdown)
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (m *Storage) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		m.messageStorage,
		m.messageMetadataStorage,
		m.approverStorage,
		m.missingMessageStorage,
		m.attachmentStorage,
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
func (m *Storage) DBStats() (solidCount int, messageCount int, avgSolidificationTime float64, missingMessageCount int) {
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
func (m *Storage) RetrieveAllTips() (tips []MessageID) {
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
