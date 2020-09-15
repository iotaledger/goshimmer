package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

const (
	// MaxMessageSize defines the maximum size of a message.
	MaxMessageSize = 64 * 1024

	// MessageIDLength defines the length of an MessageID.
	MessageIDLength = 64
)

// ContentID identifies the content of a message without its parent1/parent2 ids.
type ContentID = MessageID

// MessageID identifies a message in its entirety. Unlike the sole content id, it also incorporates
// the parent1 and parent2 ids.
type MessageID [MessageIDLength]byte

// EmptyMessageID is an empty id.
var EmptyMessageID = MessageID{}

// NewMessageID creates a new message id.
func NewMessageID(base58EncodedString string) (result MessageID, err error) {
	bytes, err := base58.Decode(base58EncodedString)
	if err != nil {
		err = fmt.Errorf("failed to decode base58 encoded string '%s': %w", base58EncodedString, err)

		return
	}

	if len(bytes) != MessageIDLength {
		err = fmt.Errorf("length of base58 formatted message id is wrong")

		return
	}

	copy(result[:], bytes)

	return
}

// MessageIDFromBytes unmarshals a message id from a sequence of bytes.
func MessageIDFromBytes(bytes []byte) (result MessageID, consumedBytes int, err error) {
	// check arguments
	if len(bytes) < MessageIDLength {
		err = fmt.Errorf("bytes not long enough to encode a valid message id")
	}

	// calculate result
	copy(result[:], bytes)

	// return the number of bytes we processed
	consumedBytes = MessageIDLength

	return
}

// MessageIDParse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func MessageIDParse(marshalUtil *marshalutil.MarshalUtil) (MessageID, error) {
	id, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return MessageIDFromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse message ID: %w", err)
		return MessageID{}, err
	}
	return id.(MessageID), nil
}

// MarshalBinary marshals the MessageID into bytes.
func (id *MessageID) MarshalBinary() (result []byte, err error) {
	return id.Bytes(), nil
}

// UnmarshalBinary unmarshals the bytes into an MessageID.
func (id *MessageID) UnmarshalBinary(data []byte) (err error) {
	copy(id[:], data)

	return
}

// Bytes returns the bytes of the MessageID.
func (id MessageID) Bytes() []byte {
	return id[:]
}

// String returns the base58 encode of the MessageID.
func (id MessageID) String() string {
	return base58.Encode(id[:])
}

// Message represents the core message for the base layer Tangle.
type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (get sent over the wire)
	parent1ID       MessageID
	parent2ID       MessageID
	issuerPublicKey ed25519.PublicKey
	issuingTime     time.Time
	sequenceNumber  uint64
	payload         Payload
	nonce           uint64
	signature       ed25519.Signature

	// derived properties
	id             *MessageID
	idMutex        sync.RWMutex
	contentID      *ContentID
	contentIDMutex sync.RWMutex
	bytes          []byte
	bytesMutex     sync.RWMutex
}

// NewMessage creates a new message with the details provided by the issuer.
func NewMessage(parent1ID MessageID, parent2ID MessageID, issuingTime time.Time, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, payload Payload, nonce uint64, signature ed25519.Signature) (result *Message) {
	return &Message{
		parent1ID:       parent1ID,
		parent2ID:       parent2ID,
		issuerPublicKey: issuerPublicKey,
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         payload,
		nonce:           nonce,
		signature:       signature,
	}
}

// MessageFromBytes parses the given bytes into a message.
func MessageFromBytes(bytes []byte) (result *Message, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MessageParse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// MessageParse parses a message from the given marshal util.
func MessageParse(marshalUtil *marshalutil.MarshalUtil) (result *Message, err error) {
	// determine read offset before starting to parse
	readOffsetStart := marshalUtil.ReadOffset()

	// parse information
	result = &Message{}
	if result.parent1ID, err = MessageIDParse(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse parent1 message ID of the message: %w", err)
		return
	}
	if result.parent2ID, err = MessageIDParse(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse parent1 message ID of the message: %w", err)
		return
	}
	if result.issuerPublicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse issuer public key of the message: %w", err)
		return
	}
	if result.issuingTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse issuing time of the message: %w", err)
		return
	}
	if result.sequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		err = fmt.Errorf("failed to parse sequence number of the message: %w", err)
		return
	}
	if result.payload, err = PayloadParse(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse payload of the message: %w", err)
		return
	}
	if result.nonce, err = marshalUtil.ReadUint64(); err != nil {
		err = fmt.Errorf("failed to parse nonce of the message: %w", err)
		return
	}
	if result.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse signature of the message: %w", err)
		return
	}

	// retrieve the number of bytes we processed
	readOffsetEnd := marshalUtil.ReadOffset()

	// store marshaled version as a copy
	result.bytes, err = marshalUtil.ReadBytes(readOffsetEnd-readOffsetStart, readOffsetStart)
	if err != nil {
		err = fmt.Errorf("error trying to copy raw source bytes: %w", err)
		return
	}

	return
}

// MessageFromObjectStorage restores a Message from the ObjectStorage.
func MessageFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	// parse the message
	message, err := MessageParse(marshalutil.New(data))
	if err != nil {
		err = fmt.Errorf("failed to parse message from object storage: %w", err)
		return
	}

	// parse the ID from they key
	id, err := MessageIDParse(marshalutil.New(key))
	if err != nil {
		err = fmt.Errorf("failed to parse message ID from object storage: %w", err)
		return
	}
	message.id = &id

	// assign result
	result = message

	return
}

// VerifySignature verifies the signature of the message.
func (m *Message) VerifySignature() bool {
	msgBytes := m.Bytes()
	signature := m.Signature()

	contentLength := len(msgBytes) - len(signature)
	content := msgBytes[:contentLength]

	return m.issuerPublicKey.VerifySignature(content, signature)
}

// ID returns the id of the message which is made up of the content id and parent1/parent2 ids.
// This id can be used for merkle proofs.
func (m *Message) ID() (result MessageID) {
	m.idMutex.RLock()

	if m.id == nil {
		m.idMutex.RUnlock()

		m.idMutex.Lock()
		defer m.idMutex.Unlock()
		if m.id != nil {
			result = *m.id
			return
		}
		result = m.calculateID()
		m.id = &result
		return
	}

	result = *m.id
	m.idMutex.RUnlock()
	return
}

// Parent1ID returns the id of the parent1 message.
func (m *Message) Parent1ID() MessageID {
	return m.parent1ID
}

// Parent2ID returns the id of the parent2 message.
func (m *Message) Parent2ID() MessageID {
	return m.parent2ID
}

// IssuerPublicKey returns the public key of the message issuer.
func (m *Message) IssuerPublicKey() ed25519.PublicKey {
	return m.issuerPublicKey
}

// IssuingTime returns the time when this message was created.
func (m *Message) IssuingTime() time.Time {
	return m.issuingTime
}

// SequenceNumber returns the sequence number of this message.
func (m *Message) SequenceNumber() uint64 {
	return m.sequenceNumber
}

// Payload returns the payload of the message.
func (m *Message) Payload() Payload {
	return m.payload
}

// Nonce returns the nonce of the message.
func (m *Message) Nonce() uint64 {
	return m.nonce
}

// Signature returns the signature of the message.
func (m *Message) Signature() ed25519.Signature {
	return m.signature
}

// ContentID returns the content id of the message which is made up of all the
// parts of the message minus the parent1 and parent2 ids.
func (m *Message) ContentID() (result ContentID) {
	m.contentIDMutex.RLock()
	if m.contentID == nil {
		m.contentIDMutex.RUnlock()

		m.contentIDMutex.Lock()
		defer m.contentIDMutex.Unlock()
		if m.contentID != nil {
			result = *m.contentID
			return
		}
		result = m.calculateContentID()
		m.contentID = &result
		return
	}

	result = *m.contentID
	m.contentIDMutex.RUnlock()
	return
}

// calculates the message id.
func (m *Message) calculateID() MessageID {
	return blake2b.Sum512(
		marshalutil.New(MessageIDLength + MessageIDLength + PayloadIDLength).
			WriteBytes(m.parent1ID.Bytes()).
			WriteBytes(m.parent2ID.Bytes()).
			WriteBytes(m.ContentID().Bytes()).
			Bytes(),
	)
}

// calculates the content id of the message.
func (m *Message) calculateContentID() ContentID {
	// compute content id from the message data (except parent1 and parent2 ids)
	return blake2b.Sum512(m.Bytes()[2*MessageIDLength:])
}

// Bytes returns the message in serialized byte form.
func (m *Message) Bytes() []byte {
	m.bytesMutex.RLock()
	if m.bytes != nil {
		defer m.bytesMutex.RUnlock()

		return m.bytes
	}

	m.bytesMutex.RUnlock()
	m.bytesMutex.RLock()
	defer m.bytesMutex.RUnlock()

	if m.bytes != nil {
		return m.bytes
	}

	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.WriteBytes(m.parent1ID.Bytes())
	marshalUtil.WriteBytes(m.parent2ID.Bytes())
	marshalUtil.WriteBytes(m.issuerPublicKey.Bytes())
	marshalUtil.WriteTime(m.issuingTime)
	marshalUtil.WriteUint64(m.sequenceNumber)
	marshalUtil.WriteBytes(m.payload.Bytes())
	marshalUtil.WriteUint64(m.nonce)
	marshalUtil.WriteBytes(m.signature.Bytes())

	m.bytes = marshalUtil.Bytes()

	return m.bytes
}

// ObjectStorageKey returns the key of the stored message object.
// This returns the bytes of the message ID.
func (m *Message) ObjectStorageKey() []byte {
	return m.ID().Bytes()
}

// ObjectStorageValue returns the value stored in object storage.
// This returns the bytes of message.
func (m *Message) ObjectStorageValue() []byte {
	return m.Bytes()
}

// Update updates the object with the values of another object.
// Since a Message is immutable, this function is not implemented and panics.
func (m *Message) Update(objectstorage.StorableObject) {
	panic("messages should never be overwritten and only stored once to optimize IO")
}

func (m *Message) String() string {
	return stringify.Struct("Message",
		stringify.StructField("id", m.ID()),
		stringify.StructField("parent1Id", m.Parent1ID()),
		stringify.StructField("parent2Id", m.Parent2ID()),
		stringify.StructField("issuer", m.IssuerPublicKey()),
		stringify.StructField("issuingTime", m.IssuingTime()),
		stringify.StructField("sequenceNumber", m.SequenceNumber()),
		stringify.StructField("payload", m.Payload()),
		stringify.StructField("nonce", m.Nonce()),
		stringify.StructField("signature", m.Signature()),
	)
}

// CachedMessage defines a cached message.
// A wrapper for a cached object.
type CachedMessage struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached message.
func (c *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{c.CachedObject.Retain()}
}

// Consume consumes the cached object and releases it when the callback is done.
// It returns true if the callback was called.
func (c *CachedMessage) Consume(consumer func(msg *Message)) bool {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	})
}

// Unwrap returns the message wrapped by the cached message.
// If the wrapped object cannot be cast to a Message or has been deleted, it returns nil.
func (c *CachedMessage) Unwrap() *Message {
	untypedMessage := c.Get()
	if untypedMessage == nil {
		return nil
	}
	typeCastedMessage := untypedMessage.(*Message)
	if typeCastedMessage == nil || typeCastedMessage.IsDeleted() {
		return nil
	}
	return typeCastedMessage
}
