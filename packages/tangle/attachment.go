package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

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

// AttachmentFromBytes unmarshals an Attachment from a sequence of bytes - it either creates a emptyTangle object or fills the
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
// returns the emptyTangle object.
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
