package transaction

import (
	"encoding/binary"
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/identity"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	"github.com/iotaledger/goshimmer/packages/stringify"

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
	issuer              *identity.Identity
	payload             payload.Payload
	bytes               []byte
	bytesMutex          sync.RWMutex
	signature           [identity.SignatureSize]byte
	signatureMutex      sync.RWMutex

	// derived properties
	id             *Id
	idMutex        sync.RWMutex
	payloadId      *payload.Id
	payloadIdMutex sync.RWMutex
}

// Allows us to "issue" a transaction.
func New(trunkTransactionId Id, branchTransactionId Id, issuer *identity.Identity, payload payload.Payload) (result *Transaction) {
	return &Transaction{
		trunkTransactionId:  trunkTransactionId,
		branchTransactionId: branchTransactionId,
		issuer:              issuer,
		payload:             payload,
	}
}

// Get's called when we restore a transaction from storage. The bytes and the content will be unmarshaled by an external
// caller (the objectStorage factory).
func FromStorage(id []byte) (result objectstorage.StorableObject) {
	var transactionId Id
	copy(transactionId[:], id)

	result = &Transaction{
		id: &transactionId,
	}

	return
}

func FromBytes(bytes []byte) (result *Transaction, err error) {
	result = &Transaction{}
	err = result.UnmarshalBinary(bytes)

	return
}

func (transaction *Transaction) VerifySignature() (result bool) {
	transactionBytes := transaction.GetBytes()

	transaction.signatureMutex.RLock()
	result = transaction.issuer.VerifySignature(transactionBytes[:len(transactionBytes)-identity.SignatureSize], transaction.signature[:])
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

func (transaction *Transaction) GetBytes() []byte {
	if result, err := transaction.MarshalBinary(); err != nil {
		panic(err)
	} else {
		return result
	}
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
	bytes := transaction.GetBytes()

	return blake2b.Sum512(bytes[2*IdLength:])
}

// Since transactions are immutable and do not get changed after being created, we cache the result of the marshaling.
func (transaction *Transaction) MarshalBinary() (result []byte, err error) {
	transaction.bytesMutex.RLock()
	if transaction.bytes == nil {
		transaction.bytesMutex.RUnlock()

		transaction.bytesMutex.Lock()
		if transaction.bytes == nil {
			var serializedPayload []byte
			if transaction.payload != nil {
				if serializedPayload, err = transaction.payload.MarshalBinary(); err != nil {
					return
				}
			}
			serializedPayloadLength := len(serializedPayload)

			result = make([]byte, IdLength+IdLength+identity.PublicKeySize+4+serializedPayloadLength+identity.SignatureSize)
			offset := 0

			copy(result[offset:], transaction.trunkTransactionId[:])
			offset += IdLength

			copy(result[offset:], transaction.branchTransactionId[:])
			offset += IdLength

			if transaction.issuer != nil {
				copy(result[offset:], transaction.issuer.PublicKey)
			}
			offset += identity.PublicKeySize

			binary.LittleEndian.PutUint32(result[offset:], transaction.payload.GetType())
			offset += 4

			if serializedPayloadLength != 0 {
				copy(result[offset:], serializedPayload)
				offset += serializedPayloadLength
			}

			if transaction.issuer != nil {
				transaction.signatureMutex.Lock()
				copy(transaction.signature[:], transaction.issuer.Sign(result[:offset]))
				transaction.signatureMutex.Unlock()
				copy(result[offset:], transaction.signature[:])
			}
			// offset += identity.SignatureSize

			transaction.bytes = result
		} else {
			result = transaction.bytes
		}
		transaction.bytesMutex.Unlock()
	} else {
		result = transaction.bytes

		transaction.bytesMutex.RUnlock()
	}

	return
}

func (transaction *Transaction) UnmarshalBinary(data []byte) (err error) {
	offset := 0

	copy(transaction.trunkTransactionId[:], data[offset:])
	offset += IdLength

	copy(transaction.branchTransactionId[:], data[offset:])
	offset += IdLength

	transaction.issuer = identity.New(data[offset : offset+identity.PublicKeySize])
	offset += identity.PublicKeySize

	payloadType := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	if transaction.payload, err = payload.GetUnmarshaler(payloadType)(data[offset : len(data)-identity.SignatureSize]); err != nil {
		return
	}
	offset += len(data) - identity.SignatureSize - offset

	copy(transaction.signature[:], data[offset:])
	// offset += identity.SignatureSize

	transaction.bytes = make([]byte, len(data))
	copy(transaction.bytes, data)

	return
}

func (transaction *Transaction) GetStorageKey() []byte {
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
	)
}
