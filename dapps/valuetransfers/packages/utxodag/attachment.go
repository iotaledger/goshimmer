package utxodag

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Attachment stores the information which transaction was attached by which payload. We need this to be able to perform
// reverse lookups from transactions to their corresponding payloads, that attach them.
type Attachment struct {
	objectstorage.StorableObjectFlags

	transactionID transaction.ID
	payloadID     payload.ID

	storageKey []byte
}

// NewAttachment creates an attachment object with the given information.
func NewAttachment(transactionID transaction.ID, payloadID payload.ID) *Attachment {
	return &Attachment{
		transactionID: transactionID,
		payloadID:     payloadID,

		storageKey: marshalutil.New(AttachmentLength).
			WriteBytes(transactionID.Bytes()).
			WriteBytes(payloadID.Bytes()).
			Bytes(),
	}
}

// AttachmentFromBytes unmarshals an Attachment from a sequence of bytes - it either creates a new object or fills the
// optionally provided one with the parsed information.
func AttachmentFromBytes(bytes []byte, optionalTargetObject ...*Attachment) (result *Attachment, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseAttachment(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseAttachment is a wrapper for simplified unmarshaling of Attachments from a byte stream using the marshalUtil package.
func ParseAttachment(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Attachment) (result *Attachment, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return AttachmentFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*Attachment)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// AttachmentFromStorageKey gets called when we restore an Attachment from the storage - it parses the key bytes and
// returns the new object.
func AttachmentFromStorageKey(key []byte, optionalTargetObject ...*Attachment) (result *Attachment, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Attachment{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to AttachmentFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.transactionID, err = transaction.ParseID(marshalUtil); err != nil {
		return
	}
	if result.payloadID, err = payload.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	result.storageKey = marshalutil.New(key[:consumedBytes]).Bytes(true)

	return
}

// TransactionID returns the transaction id of this Attachment.
func (attachment *Attachment) TransactionID() transaction.ID {
	return attachment.transactionID
}

// PayloadID returns the payload id of this Attachment.
func (attachment *Attachment) PayloadID() payload.ID {
	return attachment.payloadID
}

// Bytes marshals the Attachment into a sequence of bytes.
func (attachment *Attachment) Bytes() []byte {
	return attachment.ObjectStorageKey()
}

// String returns a human readable version of the Attachment.
func (attachment *Attachment) String() string {
	return stringify.Struct("Attachment",
		stringify.StructField("transactionId", attachment.TransactionID()),
		stringify.StructField("payloadId", attachment.PayloadID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database.
func (attachment *Attachment) ObjectStorageKey() []byte {
	return attachment.storageKey
}

// ObjectStorageValue marshals the "content part" of an Attachment to a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (attachment *Attachment) ObjectStorageValue() (data []byte) {
	return
}

// UnmarshalObjectStorageValue unmarshals the "content part" of an Attachment from a sequence of bytes. Since all of the information
// for this object are stored in its key, this method does nothing and is only required to conform with the interface.
func (attachment *Attachment) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	return
}

// Update is disabled - updates are supposed to happen through the setters (if existing).
func (attachment *Attachment) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &Attachment{}

// AttachmentLength holds the length of a marshaled Attachment in bytes.
const AttachmentLength = transaction.IDLength + payload.IDLength

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
