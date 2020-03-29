package message

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

// region Message //////////////////////////////////////////////////////////////////////////////////////////////////////

type Message struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (they are part of the transaction when being sent)
	trunkMessageId  Id
	branchMessageId Id
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
	payloadId      *payload.Id
	payloadIdMutex sync.RWMutex

	// only stored on the machine of the signer
	issuerLocalIdentity *identity.LocalIdentity
}

// New creates a new transaction with the details provided by the issuer.
func New(trunkTransactionId Id, branchTransactionId Id, localIdentity *identity.LocalIdentity, issuingTime time.Time, sequenceNumber uint64, payload payload.Payload) (result *Message) {
	return &Message{
		trunkMessageId:  trunkTransactionId,
		branchMessageId: branchTransactionId,
		issuerPublicKey: localIdentity.PublicKey(),
		issuingTime:     issuingTime,
		sequenceNumber:  sequenceNumber,
		payload:         payload,

		issuerLocalIdentity: localIdentity,
	}
}

func FromBytes(bytes []byte, optionalTargetObject ...*Message) (result *Message, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

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

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// Get's called when we restore a transaction from storage. The bytes and the content will be unmarshaled by an external
// caller (the objectStorage factory).
func StorableObjectFromKey(key []byte, optionalTargetObject ...*Message) (result objectstorage.StorableObject, err error, consumedBytes int) {
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

func (transaction *Message) VerifySignature() (result bool) {
	transactionBytes := transaction.Bytes()

	transaction.signatureMutex.RLock()
	result = transaction.issuerPublicKey.VerifySignature(transactionBytes[:len(transactionBytes)-ed25519.SignatureSize], transaction.signature)
	transaction.signatureMutex.RUnlock()

	return
}

func (transaction *Message) Id() (result Id) {
	transaction.idMutex.RLock()
	if transaction.id == nil {
		transaction.idMutex.RUnlock()

		transaction.idMutex.Lock()
		if transaction.id == nil {
			result = transaction.calculateTransactionId()

			transaction.id = &result
		} else {
			result = *transaction.id
		}
		transaction.idMutex.Unlock()
	} else {
		result = *transaction.id

		transaction.idMutex.RUnlock()
	}

	return
}

func (transaction *Message) TrunkMessageId() Id {
	return transaction.trunkMessageId
}

func (transaction *Message) BranchMessageId() Id {
	return transaction.branchMessageId
}

func (transaction *Message) IssuerPublicKey() ed25519.PublicKey {
	return transaction.issuerPublicKey
}

// IssuingTime returns the time when the transaction was created.
func (transaction *Message) IssuingTime() time.Time {
	return transaction.issuingTime
}

// SequenceNumber returns the sequence number of this transaction.
func (transaction *Message) SequenceNumber() uint64 {
	return transaction.sequenceNumber
}

func (transaction *Message) Signature() ed25519.Signature {
	return transaction.signature
}

func (transaction *Message) Payload() payload.Payload {
	return transaction.payload
}

func (transaction *Message) PayloadId() (result payload.Id) {
	transaction.payloadIdMutex.RLock()
	if transaction.payloadId == nil {
		transaction.payloadIdMutex.RUnlock()

		transaction.payloadIdMutex.Lock()
		if transaction.payloadId == nil {
			result = transaction.calculatePayloadId()

			transaction.payloadId = &result
		} else {
			result = *transaction.payloadId
		}
		transaction.payloadIdMutex.Unlock()
	} else {
		result = *transaction.payloadId

		transaction.payloadIdMutex.RUnlock()
	}

	return
}

func (transaction *Message) calculateTransactionId() Id {
	payloadId := transaction.PayloadId()

	hashBase := make([]byte, IdLength+IdLength+payload.IdLength)
	offset := 0

	copy(hashBase[offset:], transaction.trunkMessageId[:])
	offset += IdLength

	copy(hashBase[offset:], transaction.branchMessageId[:])
	offset += IdLength

	copy(hashBase[offset:], payloadId[:])
	// offset += payloadIdLength

	return blake2b.Sum512(hashBase)
}

func (transaction *Message) calculatePayloadId() payload.Id {
	bytes := transaction.Bytes()

	return blake2b.Sum512(bytes[2*IdLength:])
}

func (transaction *Message) Bytes() []byte {
	transaction.bytesMutex.RLock()
	if transaction.bytes != nil {
		defer transaction.bytesMutex.RUnlock()

		return transaction.bytes
	}

	transaction.bytesMutex.RUnlock()
	transaction.bytesMutex.RLock()
	defer transaction.bytesMutex.RUnlock()

	if transaction.bytes != nil {
		return transaction.bytes
	}

	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.WriteBytes(transaction.trunkMessageId.Bytes())
	marshalUtil.WriteBytes(transaction.branchMessageId.Bytes())
	marshalUtil.WriteBytes(transaction.issuerPublicKey.Bytes())
	marshalUtil.WriteTime(transaction.issuingTime)
	marshalUtil.WriteUint64(transaction.sequenceNumber)
	marshalUtil.WriteBytes(transaction.payload.Bytes())

	transaction.signature = transaction.issuerLocalIdentity.Sign(marshalUtil.Bytes())

	marshalUtil.WriteBytes(transaction.signature.Bytes())

	transaction.bytes = marshalUtil.Bytes()

	return transaction.bytes
}

func (transaction *Message) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	// initialize helper
	marshalUtil := marshalutil.New(data)

	// parse information
	if transaction.trunkMessageId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if transaction.branchMessageId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if transaction.issuerPublicKey, err = ed25519.ParsePublicKey(marshalUtil); err != nil {
		return
	}
	if transaction.issuingTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if transaction.sequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		return
	}
	if transaction.payload, err = payload.Parse(marshalUtil); err != nil {
		return
	}
	if transaction.signature, err = ed25519.ParseSignature(marshalUtil); err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store marshaled version
	transaction.bytes = make([]byte, consumedBytes)
	copy(transaction.bytes, data)

	return
}

func (transaction *Message) ObjectStorageKey() []byte {
	return transaction.Id().Bytes()
}

// Since transactions are immutable and do not get changed after being created, we cache the result of the marshaling.
func (transaction *Message) ObjectStorageValue() []byte {
	return transaction.Bytes()
}

func (transaction *Message) Update(other objectstorage.StorableObject) {
	panic("transactions should never be overwritten and only stored once to optimize IO")
}

func (transaction *Message) String() string {
	transactionId := transaction.Id()

	return stringify.Struct("Message",
		stringify.StructField("id", base58.Encode(transactionId[:])),
		stringify.StructField("trunkMessageId", base58.Encode(transaction.trunkMessageId[:])),
		stringify.StructField("branchMessageId", base58.Encode(transaction.branchMessageId[:])),
		stringify.StructField("issuer", base58.Encode(transaction.issuerPublicKey[:])),
		stringify.StructField("issuingTime", transaction.issuingTime),
		stringify.StructField("sequenceNumber", transaction.sequenceNumber),
		stringify.StructField("payload", transaction.payload),
		stringify.StructField("signature", transaction.signature[:]),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMessage ////////////////////////////////////////////////////////////////////////////////////////////////

type CachedMessage struct {
	objectstorage.CachedObject
}

func (cachedMessage *CachedMessage) Retain() *CachedMessage {
	return &CachedMessage{cachedMessage.CachedObject.Retain()}
}

func (cachedMessage *CachedMessage) Consume(consumer func(transaction *Message)) bool {
	return cachedMessage.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Message))
	})
}

func (cachedMessage *CachedMessage) Unwrap() *Message {
	if untypedTransaction := cachedMessage.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Message); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
