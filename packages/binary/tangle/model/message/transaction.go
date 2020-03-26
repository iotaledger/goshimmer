package message

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/mr-tron/base58"

	"golang.org/x/crypto/blake2b"
)

type Transaction struct {
	// base functionality of StorableObject
	objectstorage.StorableObjectFlags

	// core properties (they are part of the transaction when being sent)
	trunkTransactionId  Id
	branchTransactionId Id
	issuerPublicKey     ed25119.PublicKey
	issuingTime         time.Time
	sequenceNumber      uint64
	payload             payload.Payload
	bytes               []byte
	bytesMutex          sync.RWMutex
	signature           ed25119.Signature
	signatureMutex      sync.RWMutex

	// derived properties
	id             *Id
	idMutex        sync.RWMutex
	payloadId      *payload.Id
	payloadIdMutex sync.RWMutex

	// only stored on the machine of the signer
	issuerPrivateKey ed25119.PrivateKey
}

// New creates a new transaction with the details provided by the issuer.
func New(trunkTransactionId Id, branchTransactionId Id, issuerKeyPair ed25119.KeyPair, issuingTime time.Time, sequenceNumber uint64, payload payload.Payload) (result *Transaction) {
	return &Transaction{
		trunkTransactionId:  trunkTransactionId,
		branchTransactionId: branchTransactionId,
		issuerPublicKey:     issuerKeyPair.PublicKey,
		issuingTime:         issuingTime,
		sequenceNumber:      sequenceNumber,
		payload:             payload,

		issuerPrivateKey: issuerKeyPair.PrivateKey,
	}
}

func FromBytes(bytes []byte) (result *Transaction, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Transaction, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return StorableObjectFromKey(data)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*Transaction)
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
func StorableObjectFromKey(key []byte) (result objectstorage.StorableObject, err error, consumedBytes int) {
	result = &Transaction{}

	marshalUtil := marshalutil.New(key)
	*result.(*Transaction).id, err = ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (transaction *Transaction) VerifySignature() (result bool) {
	transactionBytes := transaction.Bytes()

	transaction.signatureMutex.RLock()
	result = transaction.issuerPublicKey.VerifySignature(transactionBytes[:len(transactionBytes)-ed25119.SignatureSize], transaction.signature)
	transaction.signatureMutex.RUnlock()

	return
}

func (transaction *Transaction) GetId() (result Id) {
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

func (transaction *Transaction) GetTrunkTransactionId() Id {
	return transaction.trunkTransactionId
}

func (transaction *Transaction) GetBranchTransactionId() Id {
	return transaction.branchTransactionId
}

// IssuingTime returns the time when the transaction was created.
func (transaction *Transaction) IssuingTime() time.Time {
	return transaction.issuingTime
}

// SequenceNumber returns the sequence number of this transaction.
func (transaction *Transaction) SequenceNumber() uint64 {
	return transaction.sequenceNumber
}

func (transaction *Transaction) GetPayload() payload.Payload {
	return transaction.payload
}

func (transaction *Transaction) GetPayloadId() (result payload.Id) {
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

func (transaction *Transaction) calculateTransactionId() Id {
	payloadId := transaction.GetPayloadId()

	hashBase := make([]byte, IdLength+IdLength+payload.IdLength)
	offset := 0

	copy(hashBase[offset:], transaction.trunkTransactionId[:])
	offset += IdLength

	copy(hashBase[offset:], transaction.branchTransactionId[:])
	offset += IdLength

	copy(hashBase[offset:], payloadId[:])
	// offset += payloadIdLength

	return blake2b.Sum512(hashBase)
}

func (transaction *Transaction) calculatePayloadId() payload.Id {
	bytes := transaction.Bytes()

	return blake2b.Sum512(bytes[2*IdLength:])
}

func (transaction *Transaction) Bytes() []byte {
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
	marshalUtil.WriteBytes(transaction.trunkTransactionId.Bytes())
	marshalUtil.WriteBytes(transaction.branchTransactionId.Bytes())
	marshalUtil.WriteBytes(transaction.issuerPublicKey.Bytes())
	marshalUtil.WriteTime(transaction.issuingTime)
	marshalUtil.WriteUint64(transaction.sequenceNumber)
	marshalUtil.WriteBytes(transaction.payload.Bytes())
	marshalUtil.WriteBytes(transaction.issuerPrivateKey.Sign(marshalUtil.Bytes()).Bytes())

	return marshalUtil.Bytes()
}

// Since transactions are immutable and do not get changed after being created, we cache the result of the marshaling.
func (transaction *Transaction) ObjectStorageValue() []byte {
	return transaction.Bytes()
}

func (transaction *Transaction) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	// initialize helper
	marshalUtil := marshalutil.New(data)

	// parse information
	if transaction.trunkTransactionId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if transaction.branchTransactionId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if transaction.issuerPublicKey, err = ed25119.ParsePublicKey(marshalUtil); err != nil {
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
	if transaction.signature, err = ed25119.ParseSignature(marshalUtil); err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store marshaled version
	transaction.bytes = make([]byte, consumedBytes)
	copy(transaction.bytes, data)

	return
}

func (transaction *Transaction) ObjectStorageKey() []byte {
	transactionId := transaction.GetId()

	return transactionId[:]
}

func (transaction *Transaction) Update(other objectstorage.StorableObject) {
	panic("transactions should never be overwritten and only stored once to optimize IO")
}

func (transaction *Transaction) String() string {
	transactionId := transaction.GetId()

	return stringify.Struct("Transaction",
		stringify.StructField("id", base58.Encode(transactionId[:])),
		stringify.StructField("trunkTransactionId", base58.Encode(transaction.trunkTransactionId[:])),
		stringify.StructField("trunkTransactionId", base58.Encode(transaction.branchTransactionId[:])),
		stringify.StructField("payload", transaction.payload),
	)
}
