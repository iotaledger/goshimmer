package tangle

import (
	"fmt"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
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

	// PrefixFCoB defines the storage prefix for FCoB.
	PrefixFCoB

	cacheTime = 20 * time.Second

	// DBSequenceNumber defines the db sequence number.
	DBSequenceNumber = "seq"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

// Storage represents the storage of messages.
type Storage struct {
	tangle                            *Tangle
	messageStorage                    *objectstorage.ObjectStorage
	messageMetadataStorage            *objectstorage.ObjectStorage
	approverStorage                   *objectstorage.ObjectStorage
	missingMessageStorage             *objectstorage.ObjectStorage
	attachmentStorage                 *objectstorage.ObjectStorage
	markerIndexBranchIDMappingStorage *objectstorage.ObjectStorage

	Events   *StorageEvents
	shutdown chan struct{}
}

// NewStorage creates a new Storage.
func NewStorage(tangle *Tangle) (result *Storage) {
	osFactory := objectstorage.NewFactory(tangle.Options.Store, database.PrefixMessageLayer)

	result = &Storage{
		tangle:                            tangle,
		shutdown:                          make(chan struct{}),
		messageStorage:                    osFactory.New(PrefixMessage, MessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		messageMetadataStorage:            osFactory.New(PrefixMessageMetadata, MessageMetadataFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		approverStorage:                   osFactory.New(PrefixApprovers, ApproverFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(MessageIDLength, ApproverTypeLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		missingMessageStorage:             osFactory.New(PrefixMissingMessage, MissingMessageFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		attachmentStorage:                 osFactory.New(PrefixAttachments, AttachmentFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.PartitionKey(ledgerstate.TransactionIDLength, MessageIDLength), objectstorage.LeakDetectionEnabled(false)),
		markerIndexBranchIDMappingStorage: osFactory.New(PrefixMarkerBranchIDMapping, MarkerIndexBranchIDMappingFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),

		Events: &StorageEvents{
			MessageStored:        events.NewEvent(messageIDEventHandler),
			MessageRemoved:       events.NewEvent(messageIDEventHandler),
			MissingMessageStored: events.NewEvent(messageIDEventHandler),
		},
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (s *Storage) Setup() {
	s.tangle.Parser.Events.MessageParsed.Attach(events.NewClosure(func(msgParsedEvent *MessageParsedEvent) {
		s.tangle.Storage.StoreMessage(msgParsedEvent.Message)
	}))
	s.tangle.MessageFactory.Events.MessageConstructed.Attach(events.NewClosure(s.StoreMessage))
}

// StoreMessage stores a new message to the message store.
func (s *Storage) StoreMessage(message *Message) {
	// retrieve MessageID
	messageID := message.ID()

	// store Messages only once by using the existence of the Metadata as a guard
	storedMetadata, stored := s.messageMetadataStorage.StoreIfAbsent(NewMessageMetadata(messageID))
	if !stored {
		return
	}

	// create typed version of the stored MessageMetadata
	cachedMsgMetadata := &CachedMessageMetadata{CachedObject: storedMetadata}
	defer cachedMsgMetadata.Release()

	// store Message
	cachedMessage := &CachedMessage{CachedObject: s.messageStorage.Store(message)}
	defer cachedMessage.Release()

	// TODO: approval switch: we probably need to introduce approver types
	// store approvers
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		s.approverStorage.Store(NewApprover(StrongApprover, parentMessageID, messageID)).Release()
	})
	message.ForEachWeakParent(func(parentMessageID MessageID) {
		s.approverStorage.Store(NewApprover(WeakApprover, parentMessageID, messageID)).Release()
	})

	// trigger events
	if s.missingMessageStorage.DeleteIfPresent(messageID[:]) {
		s.tangle.Storage.Events.MissingMessageStored.Trigger(messageID)
	}

	// messages are stored, trigger MessageStored event to move on next check
	s.Events.MessageStored.Trigger(message.ID())
}

// Message retrieves a message from the message store.
func (s *Storage) Message(messageID MessageID) *CachedMessage {
	return &CachedMessage{CachedObject: s.messageStorage.Load(messageID[:])}
}

// MessageMetadata retrieves the MessageMetadata with the given MessageID.
func (s *Storage) MessageMetadata(messageID MessageID, computeIfAbsentCallback ...func() *MessageMetadata) *CachedMessageMetadata {
	if len(computeIfAbsentCallback) >= 1 {
		return &CachedMessageMetadata{s.messageMetadataStorage.ComputeIfAbsent(messageID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return computeIfAbsentCallback[0]()
		})}
	}

	return &CachedMessageMetadata{CachedObject: s.messageMetadataStorage.Load(messageID[:])}
}

// Approvers retrieves the Approvers of a Message from the object storage. It is possible to provide an optional
// ApproverType to only return the corresponding Approvers.
func (s *Storage) Approvers(messageID MessageID, optionalApproverType ...ApproverType) (cachedApprovers CachedApprovers) {
	var iterationPrefix []byte
	if len(optionalApproverType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(messageID.Bytes(), optionalApproverType[0].Bytes())
	} else {
		iterationPrefix = messageID.Bytes()
	}

	cachedApprovers = make(CachedApprovers, 0)
	s.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedApprovers = append(cachedApprovers, &CachedApprover{CachedObject: cachedObject})
		return true
	}, iterationPrefix)

	return
}

// StoreMissingMessage stores a new MissingMessage entry in the object storage.
func (s *Storage) StoreMissingMessage(missingMessage *MissingMessage) (cachedMissingMessage *CachedMissingMessage, stored bool) {
	cachedObject, stored := s.missingMessageStorage.StoreIfAbsent(missingMessage)
	cachedMissingMessage = &CachedMissingMessage{CachedObject: cachedObject}

	return
}

// MissingMessages return the ids of messages in missingMessageStorage
func (s *Storage) MissingMessages() (ids []MessageID) {
	s.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			ids = append(ids, object.(*MissingMessage).messageID)
		})

		return true
	})
	return
}

// StoreAttachment stores a new attachment if not already stored.
func (s *Storage) StoreAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) (cachedAttachment *CachedAttachment, stored bool) {
	attachment, stored := s.attachmentStorage.StoreIfAbsent(NewAttachment(transactionID, messageID))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}
	return
}

// Attachments retrieves the attachment of a transaction in attachmentStorage.
func (s *Storage) Attachments(transactionID ledgerstate.TransactionID) (cachedAttachments CachedAttachments) {
	s.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedAttachments = append(cachedAttachments, &CachedAttachment{CachedObject: cachedObject})
		return true
	}, transactionID.Bytes())
	return
}

// AttachmentMessageIDs returns the messageIDs of the transaction in attachmentStorage.
func (s *Storage) AttachmentMessageIDs(transactionID ledgerstate.TransactionID) (messageIDs MessageIDs) {
	s.Attachments(transactionID).Consume(func(attachment *Attachment) {
		messageIDs = append(messageIDs, attachment.MessageID())
	})
	return
}

// DeleteMessage deletes a message and its association to approvees by un-marking the given
// message as an approver.
func (s *Storage) DeleteMessage(messageID MessageID) {
	s.Message(messageID).Consume(func(currentMsg *Message) {
		currentMsg.ForEachStrongParent(func(parentMessageID MessageID) {
			s.deleteStrongApprover(parentMessageID, messageID)
		})
		currentMsg.ForEachWeakParent(func(parentMessageID MessageID) {
			s.deleteWeakApprover(parentMessageID, messageID)
		})

		s.messageMetadataStorage.Delete(messageID[:])
		s.messageStorage.Delete(messageID[:])

		s.Events.MessageRemoved.Trigger(messageID)
	})
}

// DeleteMissingMessage deletes a message from the missingMessageStorage.
func (s *Storage) DeleteMissingMessage(messageID MessageID) {
	s.missingMessageStorage.Delete(messageID[:])
}

// MarkerIndexBranchIDMapping retrieves the MarkerIndexBranchIDMapping for the given SequenceID. It accepts an optional
// computeIfAbsent callback that can be used to dynamically create a MarkerIndexBranchIDMapping if it doesn't exist,
// yet.
func (s *Storage) MarkerIndexBranchIDMapping(sequenceID markers.SequenceID, computeIfAbsentCallback ...func(sequenceID markers.SequenceID) *MarkerIndexBranchIDMapping) *CachedMarkerIndexBranchIDMapping {
	if len(computeIfAbsentCallback) >= 1 {
		return &CachedMarkerIndexBranchIDMapping{s.messageMetadataStorage.ComputeIfAbsent(sequenceID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return computeIfAbsentCallback[0](sequenceID)
		})}
	}

	return &CachedMarkerIndexBranchIDMapping{CachedObject: s.messageMetadataStorage.Load(sequenceID.Bytes())}
}

// deleteStrongApprover deletes an Approver from the object storage that was created by a strong parent.
func (s *Storage) deleteStrongApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	s.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), StrongApprover.Bytes(), approvingMessage.Bytes()))
}

// deleteWeakApprover deletes an Approver from the object storage that was created by a weak parent.
func (s *Storage) deleteWeakApprover(approvedMessageID MessageID, approvingMessage MessageID) {
	s.approverStorage.Delete(byteutils.ConcatBytes(approvedMessageID.Bytes(), WeakApprover.Bytes(), approvingMessage.Bytes()))
}

// Shutdown marks the tangle as stopped, so it will not accept any new messages (waits for all backgroundTasks to finish).
func (s *Storage) Shutdown() {
	s.messageStorage.Shutdown()
	s.messageMetadataStorage.Shutdown()
	s.approverStorage.Shutdown()
	s.missingMessageStorage.Shutdown()
	s.attachmentStorage.Shutdown()

	close(s.shutdown)
}

// Prune resets the database and deletes all objects (good for testing or "node resets").
func (s *Storage) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		s.messageStorage,
		s.messageMetadataStorage,
		s.approverStorage,
		s.missingMessageStorage,
		s.attachmentStorage,
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
func (s *Storage) DBStats() (solidCount int, messageCount int, avgSolidificationTime float64, missingMessageCount int) {
	var sumSolidificationTime time.Duration
	s.messageMetadataStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
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
	s.missingMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
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
func (s *Storage) RetrieveAllTips() (tips []MessageID) {
	s.messageMetadataStorage.ForEach(func(key []byte, cachedMessage objectstorage.CachedObject) bool {
		cachedMessage.Consume(func(object objectstorage.StorableObject) {
			messageMetadata := object.(*MessageMetadata)
			if messageMetadata != nil && messageMetadata.IsSolid() {
				cachedApprovers := s.Approvers(messageMetadata.messageID)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorageEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// StorageEvents represents events happening on the message store.
type StorageEvents struct {
	// Fired when a message has been stored.
	MessageStored *events.Event

	// Fired when a message was removed from storage.
	MessageRemoved *events.Event

	// Fired when a message which was previously marked as missing was received.
	MissingMessageStored *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApproverType /////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// StrongApprover is the ApproverType that represents references formed by strong parents.
	StrongApprover ApproverType = iota

	// WeakApprover is the ApproverType that represents references formed by weak parents.
	WeakApprover
)

// ApproverTypeLength contains the amount of bytes that a marshaled version of the ApproverType contains.
const ApproverTypeLength = 1

// ApproverType is a type that represents the different kind of reverse mapping that we have for references formed by
// strong and weak parents.
type ApproverType uint8

// ApproverTypeFromBytes unmarshals an ApproverType from a sequence of bytes.
func ApproverTypeFromBytes(bytes []byte) (approverType ApproverType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ApproverTypeFromMarshalUtil unmarshals an ApproverType using a MarshalUtil (for easier unmarshaling).
func ApproverTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (approverType ApproverType, err error) {
	untypedApproverType, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse ApproverType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if approverType = ApproverType(untypedApproverType); approverType != StrongApprover && approverType != WeakApprover {
		err = xerrors.Errorf("invalid ApproverType(%X): %w", approverType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns a marshaled version of the ApproverType.
func (a ApproverType) Bytes() []byte {
	return []byte{byte(a)}
}

// String returns a human readable version of the ApproverType.
func (a ApproverType) String() string {
	switch a {
	case StrongApprover:
		return "ApproverType(StrongApprover)"
	case WeakApprover:
		return "ApproverType(WeakApprover)"
	default:
		return fmt.Sprintf("ApproverType(%X)", uint8(a))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Approver /////////////////////////////////////////////////////////////////////////////////////////////////////

// Approver is an approver of a given referenced message.
type Approver struct {
	// approverType defines if the reference was create by a strong or a weak parent reference.
	approverType ApproverType

	// the message which got referenced by the approver message.
	referencedMessageID MessageID

	// the message which approved/referenced the given referenced message.
	approverMessageID MessageID

	objectstorage.StorableObjectFlags
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(approverType ApproverType, referencedMessageID MessageID, approverMessageID MessageID) *Approver {
	approver := &Approver{
		approverType:        approverType,
		referencedMessageID: referencedMessageID,
		approverMessageID:   approverMessageID,
	}
	return approver
}

// ApproverFromBytes parses the given bytes into an approver.
func ApproverFromBytes(bytes []byte) (result *Approver, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ApproverFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// ApproverFromMarshalUtil parses a new approver from the given marshal util.
func ApproverFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Approver, err error) {
	result = &Approver{}
	if result.referencedMessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse referenced MessageID from MarshalUtil: %w", err)
		return
	}
	if result.approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	if result.approverMessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse approver MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// ApproverFromObjectStorage is the factory method for Approvers stored in the ObjectStorage.
func ApproverFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ApproverFromBytes(key); err != nil {
		err = xerrors.Errorf("failed to parse Approver from bytes: %w", err)
		return
	}

	return
}

// Type returns the type of the Approver reference.
func (a *Approver) Type() ApproverType {
	return a.approverType
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (a *Approver) ReferencedMessageID() MessageID {
	return a.referencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (a *Approver) ApproverMessageID() MessageID {
	return a.approverMessageID
}

// Bytes returns the bytes of the approver.
func (a *Approver) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns the string representation of the approver.
func (a *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageID", a.ReferencedMessageID()),
		stringify.StructField("approverMessageID", a.ApproverMessageID()),
	)
}

// ObjectStorageKey marshals the keys of the stored approver into a byte array.
// This includes the referencedMessageID and the approverMessageID.
func (a *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		Write(a.referencedMessageID).
		Write(a.approverType).
		Write(a.approverMessageID).
		Bytes()
}

// ObjectStorageValue returns the value of the stored approver object.
func (a *Approver) ObjectStorageValue() (result []byte) {
	return
}

// Update updates the approver.
// This should should never happen and will panic if attempted.
func (a *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = &Approver{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedApprover ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedApprover is a wrapper for a stored cached object representing an approver.
type CachedApprover struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the cached approver into the underlying approver.
// If stored object cannot be cast into an approver or has been deleted, it returns nil.
func (c *CachedApprover) Unwrap() *Approver {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Approver)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject

}

// Consume consumes the cachedApprover.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *CachedApprover) Consume(consumer func(approver *Approver), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Approver))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedApprovers //////////////////////////////////////////////////////////////////////////////////////////////

// CachedApprovers defines a slice of *CachedApprover.
type CachedApprovers []*CachedApprover

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedApprovers) Unwrap() (unwrappedApprovers []*Approver) {
	unwrappedApprovers = make([]*Approver, len(c))
	for i, cachedApprover := range c {
		untypedObject := cachedApprover.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*Approver)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedApprovers[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedApprovers) Consume(consumer func(approver *Approver), forceRelease ...bool) (consumed bool) {
	for _, cachedApprover := range c {
		consumed = cachedApprover.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedApprovers) Release(force ...bool) {
	for _, cachedApprover := range c {
		cachedApprover.Release(force...)
	}
}

// String returns a human readable version of the CachedApprovers.
func (c CachedApprovers) String() string {
	structBuilder := stringify.StructBuilder("CachedApprovers")
	for i, cachedApprover := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedApprover))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Attachment ///////////////////////////////////////////////////////////////////////////////////////////////////

// Attachment stores the information which transaction was attached by which message. We need this to be able to perform
// reverse lookups from transactions to their corresponding messages that attach them.
type Attachment struct {
	objectstorage.StorableObjectFlags

	transactionID ledgerstate.TransactionID
	messageID     MessageID
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID ledgerstate.TransactionID, messageID MessageID) *Attachment {
	return &Attachment{
		transactionID: transactionID,
		messageID:     messageID,
	}
}

// AttachmentFromBytes unmarshals an Attachment from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func AttachmentFromBytes(bytes []byte) (result *Attachment, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseAttachment(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseAttachment is a wrapper for simplified unmarshaling of Attachments from a byte stream using the marshalUtil package.
func ParseAttachment(marshalUtil *marshalutil.MarshalUtil) (result *Attachment, err error) {
	result = &Attachment{}
	if result.transactionID, err = ledgerstate.TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse transaction ID in attachment: %w", err)
		return
	}
	if result.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse message ID in attachment: %w", err)
		return
	}

	return
}

// AttachmentFromObjectStorage gets called when we restore an Attachment from the storage - it parses the key bytes and
// returns the new object.
func AttachmentFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	result, _, err = AttachmentFromBytes(key)
	if err != nil {
		err = xerrors.Errorf("failed to parse attachment from object storage: %w", err)
	}

	return
}

// TransactionID returns the transactionID of this Attachment.
func (a *Attachment) TransactionID() ledgerstate.TransactionID {
	return a.transactionID
}

// MessageID returns the messageID of this Attachment.
func (a *Attachment) MessageID() MessageID {
	return a.messageID
}

// Bytes marshals the Attachment into a sequence of bytes.
func (a *Attachment) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns a human readable version of the Attachment.
func (a *Attachment) String() string {
	return stringify.Struct("Attachment",
		stringify.StructField("transactionId", a.TransactionID()),
		stringify.StructField("messageID", a.MessageID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database.
func (a *Attachment) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(a.transactionID.Bytes(), a.MessageID().Bytes())
}

// ObjectStorageValue marshals the "content part" of an Attachment to a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (a *Attachment) ObjectStorageValue() (data []byte) {
	return
}

// Update is disabled - updates are supposed to happen through the setters (if existing).
func (a *Attachment) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &Attachment{}

// AttachmentLength holds the length of a marshaled Attachment in bytes.
const AttachmentLength = ledgerstate.TransactionIDLength + MessageIDLength

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedAttachment /////////////////////////////////////////////////////////////////////////////////////////////

// CachedAttachment is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedAttachment struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedAttachment *CachedAttachment) Retain() *CachedAttachment {
	return &CachedAttachment{cachedAttachment.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedAttachment *CachedAttachment) Unwrap() *Attachment {
	untypedObject := cachedAttachment.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Attachment)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedAttachment *CachedAttachment) Consume(consumer func(attachment *Attachment)) (consumed bool) {
	return cachedAttachment.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Attachment))
	})
}

// CachedAttachments represents a collection of CachedAttachments.
type CachedAttachments []*CachedAttachment

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedAttachments CachedAttachments) Consume(consumer func(attachment *Attachment)) (consumed bool) {
	for _, cachedAttachment := range cachedAttachments {
		consumed = cachedAttachment.Consume(func(output *Attachment) {
			consumer(output)
		}) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MissingMessage ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingMessage represents a missing message.
type MissingMessage struct {
	objectstorage.StorableObjectFlags

	messageID    MessageID
	missingSince time.Time
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID MessageID) *MissingMessage {
	return &MissingMessage{
		messageID:    messageID,
		missingSince: time.Now(),
	}
}

// MissingMessageFromBytes parses the given bytes into a MissingMessage.
func MissingMessageFromBytes(bytes []byte) (result *MissingMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MissingMessageFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MissingMessageFromMarshalUtil parses a MissingMessage from the given MarshalUtil.
func MissingMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *MissingMessage, err error) {
	result = &MissingMessage{}

	if result.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of missing message: %w", err)
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse missingSince of missing message: %w", err)
		return
	}

	return
}

// MissingMessageFromObjectStorage restores a MissingMessage from the ObjectStorage.
func MissingMessageFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	result, _, err = MissingMessageFromBytes(byteutils.ConcatBytes(key, data))
	if err != nil {
		err = fmt.Errorf("failed to parse missing message from object storage: %w", err)
		return
	}

	return
}

// MessageID returns the id of the message.
func (m *MissingMessage) MessageID() MessageID {
	return m.messageID
}

// MissingSince returns the time since when this message is missing.
func (m *MissingMessage) MissingSince() time.Time {
	return m.missingSince
}

// Bytes returns a marshaled version of this MissingMessage.
func (m *MissingMessage) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// Update update the missing message.
// It should never happen and will panic if called.
func (m *MissingMessage) Update(other objectstorage.StorableObject) {
	panic("missing messages should never be overwritten and only stored once to optimize IO")
}

// ObjectStorageKey returns the key of the stored missing message.
// This returns the bytes of the messageID of the missing message.
func (m *MissingMessage) ObjectStorageKey() []byte {
	return m.messageID[:]
}

// ObjectStorageValue returns the value of the stored missing message.
func (m *MissingMessage) ObjectStorageValue() (result []byte) {
	result, err := m.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMissingMessage /////////////////////////////////////////////////////////////////////////////////////////

// CachedMissingMessage is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedMissingMessage struct {
	objectstorage.CachedObject
}

// ID returns the MissingMessageID of the requested MissingMessage.
func (c *CachedMissingMessage) ID() (id MessageID) {
	id, _, err := MessageIDFromBytes(c.Key())
	if err != nil {
		panic(err)
	}

	return
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMissingMessage) Retain() *CachedMissingMessage {
	return &CachedMissingMessage{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMissingMessage) Unwrap() *MissingMessage {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MissingMessage)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMissingMessage) Consume(consumer func(missingMessage *MissingMessage), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MissingMessage))
	}, forceRelease...)
}

// String returns a human readable version of the CachedMissingMessage.
func (c *CachedMissingMessage) String() string {
	return stringify.Struct("CachedMissingMessage",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
