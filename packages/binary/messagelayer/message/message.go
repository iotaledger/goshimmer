package message

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
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
	trunkId         Id
	branchId        Id
	issuerPublicKey ed25519.PublicKey
	issuingTime     time.Time
	sequenceNumber  uint64
	payload         payload.Payload
	bytes           []byte
	bytesMutex      sync.RWMutex
	signature       ed25519.Signature
	signatureMutex  sync.RWMutex

	// derived properties
	id             *Id
	idMutex        sync.RWMutex
	contentId      *ContentId
	contentIdMutex sync.RWMutex

	// only stored on the machine of the signer
	issuerLocalIdentity *identity.LocalIdentity
}

// New creates a new message with the details provided by the issuer.
func New(trunkMessageId Id, branchMessageId Id, localIdentity *identity.LocalIdentity, issuingTime time.Time, sequenceNumber uint64, payload payload.Payload) (result *Message) {
	return &Message{
		trunkId:         trunkMessageId,
		branchId:        branchMessageId,
		issuerPublicKey: localIdentity.PublicKey(),
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         payload,

		issuerLocalIdentity: localIdentity,
	}
}

// FromBytes parses the given bytes into a message.
func FromBytes(bytes []byte, optionalTargetObject ...*Message) (result *Message, err error, consumedBytes int) {
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

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

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
	if id, idErr := ParseId(marshalUtil); idErr != nil {
		err = idErr

		return
	} else {
		result.(*Message).id = &id
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// VerifySignature verifies the signature of the message.
func (message *Message) VerifySignature() bool {
	msgBytes := message.Bytes()
	message.signatureMutex.RLock()
	valid := message.issuerPublicKey.VerifySignature(msgBytes[:len(msgBytes)-ed25519.SignatureSize], message.Signature())
	message.signatureMutex.RUnlock()
	return valid
}

// ID returns the id of the message which is made up of the content id and trunk/branch ids.
// This id can be used for merkle proofs.
func (message *Message) Id() (result Id) {
	message.idMutex.RLock()

	if message.id == nil {
		message.idMutex.RUnlock()

		message.idMutex.Lock()
		defer message.idMutex.Unlock()
		if message.id != nil {
			result = *message.id
			return
		}
		result = message.calculateId()
		message.id = &result
		return
	}

	result = *message.id
	message.idMutex.RUnlock()
	return
}

// TrunkID returns the id of the trunk message.
func (message *Message) TrunkId() Id {
	return message.trunkId
}

// BranchID returns the id of the branch message.
func (message *Message) BranchId() Id {
	return message.branchId
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

// Signature returns the signature of the message.
func (message *Message) Signature() ed25519.Signature {
	message.signatureMutex.RLock()
	defer message.signatureMutex.RUnlock()

	if message.signature == ed25519.EmptySignature {
		// unlock the signatureMutex so Bytes() can write the Signature
		message.signatureMutex.RUnlock()
		message.Bytes()
		message.signatureMutex.RLock()
	}

	return message.signature
}

// Payload returns the payload of the message.
func (message *Message) Payload() payload.Payload {
	return message.payload
}

// ContentId returns the content id of the message which is made up of all the
// parts of the message minus the trunk and branch ids.
func (message *Message) ContentId() (result ContentId) {
	message.contentIdMutex.RLock()
	if message.contentId == nil {
		message.contentIdMutex.RUnlock()

		message.contentIdMutex.Lock()
		defer message.contentIdMutex.Unlock()
		if message.contentId != nil {
			result = *message.contentId
			return
		}
		result = message.calculateContentId()
		message.contentId = &result
		return
	}

	result = *message.contentId
	message.contentIdMutex.RUnlock()
	return
}

// calculates the message id.
func (message *Message) calculateId() Id {
	return blake2b.Sum512(
		marshalutil.New(IdLength + IdLength + payload.IdLength).
			WriteBytes(message.trunkId.Bytes()).
			WriteBytes(message.branchId.Bytes()).
			WriteBytes(message.ContentId().Bytes()).
			Bytes(),
	)
}

// calculates the content id of the message.
func (message *Message) calculateContentId() ContentId {
	// compute content id from the message data (except trunk and branch ids)
	return blake2b.Sum512(message.Bytes()[2*IdLength:])
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
	marshalUtil.WriteBytes(message.trunkId.Bytes())
	marshalUtil.WriteBytes(message.branchId.Bytes())
	marshalUtil.WriteBytes(message.issuerPublicKey.Bytes())
	marshalUtil.WriteTime(message.issuingTime)
	marshalUtil.WriteUint64(message.sequenceNumber)
	marshalUtil.WriteBytes(message.payload.Bytes())

	message.signatureMutex.Lock()
	message.signature = message.issuerLocalIdentity.Sign(marshalUtil.Bytes())
	message.signatureMutex.Unlock()

	marshalUtil.WriteBytes(message.signature.Bytes())

	message.bytes = marshalUtil.Bytes()

	return message.bytes
}

func (message *Message) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(data)

	// parse information
	if message.trunkId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if message.branchId, err = ParseId(marshalUtil); err != nil {
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

func (message *Message) ObjectStorageKey() []byte {
	return message.Id().Bytes()
}

// Since messages are immutable and do not get changed after being created, we cache the result of the marshaling.
func (message *Message) ObjectStorageValue() []byte {
	return message.Bytes()
}

func (message *Message) Update(other objectstorage.StorableObject) {
	panic("messages should never be overwritten and only stored once to optimize IO")
}

func (message *Message) String() string {
	return stringify.Struct("Message",
		stringify.StructField("id", message.Id()),
		stringify.StructField("trunkMessageId", message.TrunkId()),
		stringify.StructField("branchMessageId", message.BranchId()),
		stringify.StructField("issuer", message.IssuerPublicKey()),
		stringify.StructField("issuingTime", message.IssuingTime()),
		stringify.StructField("sequenceNumber", message.SequenceNumber()),
		stringify.StructField("payload", message.Payload()),
		stringify.StructField("signature", message.Signature()),
	)
}

type CachedMessage struct {
	objectstorage.CachedObject
}

func (cachedMessage *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{cachedMessage.CachedObject.Retain()}
}

func (cachedMessage *CachedMessage) Consume(consumer func(msg *Message)) bool {
	return cachedMessage.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	})
}

func (cachedMessage *CachedMessage) Unwrap() *Message {
	if untypedMessage := cachedMessage.Get(); untypedMessage == nil {
		return nil
	} else {
		if typeCastedMessage := untypedMessage.(*Message); typeCastedMessage == nil || typeCastedMessage.IsDeleted() {
			return nil
		} else {
			return typeCastedMessage
		}
	}
}
