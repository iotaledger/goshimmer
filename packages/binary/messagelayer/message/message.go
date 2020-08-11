package message

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

// Message represents the core message for the base layer Tangle.
type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	trunkID         ID
	branchID        ID
	issuerPublicKey ed25519.PublicKey
	issuingTime     time.Time
	sequenceNumber  uint64
	payload         payload.Payload
	nonce           uint64
	signature       ed25519.Signature

	// derived properties
	id             *ID
	idMutex        sync.RWMutex
	contentID      *ContentID
	contentIDMutex sync.RWMutex
	bytes          []byte
	bytesMutex     sync.RWMutex
}

// New creates a new message with the details provided by the issuer.
func New(trunkID ID, branchID ID, issuingTime time.Time, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, payload payload.Payload, nonce uint64, signature ed25519.Signature) (result *Message) {
	return &Message{
		trunkID:         trunkID,
		branchID:        branchID,
		issuerPublicKey: issuerPublicKey,
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         payload,
		nonce:           nonce,
		signature:       signature,
	}
}

// FromBytes parses the given bytes into a message.
func FromBytes(bytes []byte, optionalTargetObject ...*Message) (result *Message, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// Parse parses a message from the given marshal util.
func Parse(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Message) (result *Message, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Message{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to Parse")
	}

	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// StorableObjectFromKey gets called when we restore a message from storage.
// The bytes and the content will be unmarshaled by an external caller (the objectStorage factory).
func StorableObjectFromKey(key []byte, optionalTargetObject ...*Message) (result objectstorage.StorableObject, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Message{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to StorableObjectFromKey")
	}

	marshalUtil := marshalutil.New(key)
	id, idErr := ParseID(marshalUtil)
	if idErr != nil {
		err = idErr

		return
	}
	result.(*Message).id = &id
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// VerifySignature verifies the signature of the message.
func (message *Message) VerifySignature() bool {
	msgBytes := message.Bytes()
	signature := message.Signature()

	contentLength := len(msgBytes) - len(signature)
	content := msgBytes[:contentLength]

	return message.issuerPublicKey.VerifySignature(content, signature)
}

// ID returns the id of the message which is made up of the content id and trunk/branch ids.
// This id can be used for merkle proofs.
func (message *Message) ID() (result ID) {
	message.idMutex.RLock()

	if message.id == nil {
		message.idMutex.RUnlock()

		message.idMutex.Lock()
		defer message.idMutex.Unlock()
		if message.id != nil {
			result = *message.id
			return
		}
		result = message.calculateID()
		message.id = &result
		return
	}

	result = *message.id
	message.idMutex.RUnlock()
	return
}

// TrunkID returns the id of the trunk message.
func (message *Message) TrunkID() ID {
	return message.trunkID
}

// BranchID returns the id of the branch message.
func (message *Message) BranchID() ID {
	return message.branchID
}

// IssuerPublicKey returns the public key of the message issuer.
func (message *Message) IssuerPublicKey() ed25519.PublicKey {
	return message.issuerPublicKey
}

// IssuingTime returns the time when this message was created.
func (message *Message) IssuingTime() time.Time {
	return message.issuingTime
}

// SequenceNumber returns the sequence number of this message.
func (message *Message) SequenceNumber() uint64 {
	return message.sequenceNumber
}

// Payload returns the payload of the message.
func (message *Message) Payload() payload.Payload {
	return message.payload
}

// Nonce returns the nonce of the message.
func (message *Message) Nonce() uint64 {
	return message.nonce
}

// Signature returns the signature of the message.
func (message *Message) Signature() ed25519.Signature {
	return message.signature
}

// ContentID returns the content id of the message which is made up of all the
// parts of the message minus the trunk and branch ids.
func (message *Message) ContentID() (result ContentID) {
	message.contentIDMutex.RLock()
	if message.contentID == nil {
		message.contentIDMutex.RUnlock()

		message.contentIDMutex.Lock()
		defer message.contentIDMutex.Unlock()
		if message.contentID != nil {
			result = *message.contentID
			return
		}
		result = message.calculateContentID()
		message.contentID = &result
		return
	}

	result = *message.contentID
	message.contentIDMutex.RUnlock()
	return
}

// calculates the message id.
func (message *Message) calculateID() ID {
	return blake2b.Sum512(
		marshalutil.New(IDLength + IDLength + payload.IDLength).
			WriteBytes(message.trunkID.Bytes()).
			WriteBytes(message.branchID.Bytes()).
			WriteBytes(message.ContentID().Bytes()).
			Bytes(),
	)
}

// calculates the content id of the message.
func (message *Message) calculateContentID() ContentID {
	// compute content id from the message data (except trunk and branch ids)
	return blake2b.Sum512(message.Bytes()[2*IDLength:])
}

// Bytes returns the message in serialized byte form.
func (message *Message) Bytes() []byte {
	message.bytesMutex.RLock()
	if message.bytes != nil {
		defer message.bytesMutex.RUnlock()

		return message.bytes
	}

	message.bytesMutex.RUnlock()
	message.bytesMutex.RLock()
	defer message.bytesMutex.RUnlock()

	if message.bytes != nil {
		return message.bytes
	}

	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.WriteBytes(message.trunkID.Bytes())
	marshalUtil.WriteBytes(message.branchID.Bytes())
	marshalUtil.WriteBytes(message.issuerPublicKey.Bytes())
	marshalUtil.WriteTime(message.issuingTime)
	marshalUtil.WriteUint64(message.sequenceNumber)
	marshalUtil.WriteBytes(message.payload.Bytes())
	marshalUtil.WriteUint64(message.nonce)
	marshalUtil.WriteBytes(message.signature.Bytes())

	message.bytes = marshalUtil.Bytes()

	return message.bytes
}

// UnmarshalObjectStorageValue unmarshals the bytes into a message.
func (message *Message) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(data)

	// parse information
	if message.trunkID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if message.branchID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if message.issuerPublicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		return
	}
	if message.issuingTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if message.sequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		return
	}
	if message.payload, err = payload.Parse(marshalUtil); err != nil {
		return
	}
	if message.nonce, err = marshalUtil.ReadUint64(); err != nil {
		return
	}
	if message.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store marshaled version
	message.bytes = make([]byte, consumedBytes)
	copy(message.bytes, data)

	return
}

// ObjectStorageKey returns the key of the stored message object.
// This returns the bytes of the message ID.
func (message *Message) ObjectStorageKey() []byte {
	return message.ID().Bytes()
}

// ObjectStorageValue returns the value stored in object storage.
// This returns the bytes of message.
func (message *Message) ObjectStorageValue() []byte {
	return message.Bytes()
}

// Update updates the object with the values of another object.
// Since a Message is immutable, this function is not implemented and panics.
func (message *Message) Update(objectstorage.StorableObject) {
	panic("messages should never be overwritten and only stored once to optimize IO")
}

func (message *Message) String() string {
	return stringify.Struct("Message",
		stringify.StructField("id", message.ID()),
		stringify.StructField("trunkId", message.TrunkID()),
		stringify.StructField("branchId", message.BranchID()),
		stringify.StructField("issuer", message.IssuerPublicKey()),
		stringify.StructField("issuingTime", message.IssuingTime()),
		stringify.StructField("sequenceNumber", message.SequenceNumber()),
		stringify.StructField("payload", message.Payload()),
		stringify.StructField("nonce", message.Nonce()),
		stringify.StructField("signature", message.Signature()),
	)
}

// CachedMessage defines a cached message.
// A wrapper for a cached object.
type CachedMessage struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached message.
func (cachedMessage *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{cachedMessage.CachedObject.Retain()}
}

// Consume consumes the cached object and releases it when the callback is done.
// It returns true if the callback was called.
func (cachedMessage *CachedMessage) Consume(consumer func(msg *Message)) bool {
	return cachedMessage.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	})
}

// Unwrap returns the message wrapped by the cached message.
// If the wrapped object cannot be cast to a Message or has been deleted, it returns nil.
func (cachedMessage *CachedMessage) Unwrap() *Message {
	untypedMessage := cachedMessage.Get()
	if untypedMessage == nil {
		return nil
	}
	typeCastedMessage := untypedMessage.(*Message)
	if typeCastedMessage == nil || typeCastedMessage.IsDeleted() {
		return nil
	}
	return typeCastedMessage
}
