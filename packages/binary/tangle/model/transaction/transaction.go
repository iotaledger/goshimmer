package transaction

import (
	"sync"

	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"

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
	issuerPrivateKey    ed25119.PrivateKey
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
}

// Allows us to "issue" a transaction.
func New(trunkTransactionId Id, branchTransactionId Id, issuerKeyPair ed25119.KeyPair, payload payload.Payload) (result *Transaction) {
	return &Transaction{
		trunkTransactionId:  trunkTransactionId,
		branchTransactionId: branchTransactionId,
		issuerPublicKey:     issuerKeyPair.PublicKey,
		issuerPrivateKey:    issuerKeyPair.PrivateKey,
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

func FromBytes(bytes []byte, optionalTargetObject ...*Transaction) (result *Transaction, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Transaction{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// parse information
	if result.trunkTransactionId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if result.branchTransactionId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if result.issuerPublicKey, err = ed25119.ParsePublicKey(marshalUtil); err != nil {
		return
	}
	if result.payload, err = payload.Parse(marshalUtil); err != nil {
		return
	}

	// read signature
	copy(result.signature[:], marshalUtil.ReadRemainingBytes())

	// store marshaled version
	result.bytes = make([]byte, len(bytes))
	copy(result.bytes, bytes)

	// return the number of bytes we processed
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
	marshalUtil.WriteBytes(transaction.payload.Bytes())
	marshalUtil.WriteBytes(transaction.issuerPrivateKey.Sign(marshalUtil.Bytes()).Bytes())

	return marshalUtil.Bytes()
}

// Since transactions are immutable and do not get changed after being created, we cache the result of the marshaling.
func (transaction *Transaction) MarshalBinary() (result []byte, err error) {
	return transaction.Bytes(), nil
}

func (transaction *Transaction) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, transaction)

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
		stringify.StructField("payload", transaction.payload),
	)
}
