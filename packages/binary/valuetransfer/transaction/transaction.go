package transaction

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address/signaturescheme"
)

// region IMPLEMENT Transaction ////////////////////////////////////////////////////////////////////////////////////////////

type Transaction struct {
	objectstorage.StorableObjectFlags

	inputs     *Inputs
	outputs    *Outputs
	signatures *Signatures

	id      *Id
	idMutex sync.RWMutex

	essenceBytes      []byte
	essenceBytesMutex sync.RWMutex

	signatureBytes      []byte
	signatureBytesMutex sync.RWMutex

	dataPayload      []byte
	dataPayloadMutex sync.RWMutex

	bytes      []byte
	bytesMutex sync.RWMutex
}

func New(inputs *Inputs, outputs *Outputs) *Transaction {
	return &Transaction{
		inputs:     inputs,
		outputs:    outputs,
		signatures: NewSignatures(),
	}
}

func FromBytes(bytes []byte, optionalTargetObject ...*Transaction) (result *Transaction, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func FromStorageKey(key []byte, optionalTargetObject ...*Transaction) (result objectstorage.StorableObject, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Transaction{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromStorageKey")
	}

	marshalUtil := marshalutil.New(key)
	id, err := ParseId(marshalUtil)
	if err != nil {
		return
	}
	result.(*Transaction).id = &id

	return
}

func Parse(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Transaction) (result *Transaction, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Transaction{}
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

func (transaction *Transaction) Id() Id {
	// acquire lock for reading id
	transaction.idMutex.RLock()

	// return if id has been calculated already
	if transaction.id != nil {
		defer transaction.idMutex.RUnlock()

		return *transaction.id
	}

	// switch to write lock
	transaction.idMutex.RUnlock()
	transaction.idMutex.Lock()
	defer transaction.idMutex.Unlock()

	// return if id has been calculated in the mean time
	if transaction.id != nil {
		return *transaction.id
	}

	// otherwise calculate the id
	idBytes := blake2b.Sum256(transaction.Bytes())
	id, err, _ := IdFromBytes(idBytes[:])
	if err != nil {
		panic(err)
	}

	// cache result for later calls
	transaction.id = &id

	return id
}

func (transaction *Transaction) Inputs() *Inputs {
	return transaction.inputs
}

func (transaction *Transaction) Outputs() *Outputs {
	return transaction.outputs
}

func (transaction *Transaction) SignaturesValid() bool {
	signaturesValid := true
	transaction.inputs.ForEachAddress(func(address address.Address) bool {
		if signature, exists := transaction.signatures.Get(address); !exists || !signature.IsValid(transaction.EssenceBytes()) {
			signaturesValid = false

			return false
		}

		return true
	})

	return signaturesValid
}

func (transaction *Transaction) EssenceBytes() []byte {
	// acquire read lock on essenceBytes
	transaction.essenceBytesMutex.RLock()

	// return essenceBytes if the object has been marshaled already
	if transaction.essenceBytes != nil {
		defer transaction.essenceBytesMutex.RUnlock()

		return transaction.essenceBytes
	}

	// switch to write lock
	transaction.essenceBytesMutex.RUnlock()
	transaction.essenceBytesMutex.Lock()
	defer transaction.essenceBytesMutex.Unlock()

	// return essenceBytes if the object has been marshaled in the mean time
	if essenceBytes := transaction.essenceBytes; essenceBytes != nil {
		return essenceBytes
	}

	// create marshal helper
	marshalUtil := marshalutil.New()

	// marshal inputs
	marshalUtil.WriteBytes(transaction.inputs.Bytes())

	// marshal outputs
	marshalUtil.WriteBytes(transaction.outputs.Bytes())

	// marshal dataPayload size
	marshalUtil.WriteUint32(transaction.DataPayloadSize())

	// marshal dataPayload data
	marshalUtil.WriteBytes(transaction.dataPayload)

	// store marshaled result
	transaction.essenceBytes = marshalUtil.Bytes()

	return transaction.essenceBytes
}

func (transaction *Transaction) SignatureBytes() []byte {
	transaction.signatureBytesMutex.RLock()
	if transaction.signatureBytes != nil {
		defer transaction.signatureBytesMutex.RUnlock()

		return transaction.signatureBytes
	}

	transaction.signatureBytesMutex.RUnlock()
	transaction.signatureBytesMutex.Lock()
	defer transaction.signatureBytesMutex.Unlock()

	if transaction.signatureBytes != nil {
		return transaction.signatureBytes
	}

	// generate signatures
	transaction.signatureBytes = transaction.signatures.Bytes()

	return transaction.signatureBytes
}

func (transaction *Transaction) Bytes() []byte {
	// acquire read lock on bytes
	transaction.bytesMutex.RLock()

	// return bytes if the object has been marshaled already
	if transaction.bytes != nil {
		defer transaction.bytesMutex.RUnlock()

		return transaction.bytes
	}

	// switch to write lock
	transaction.bytesMutex.RUnlock()
	transaction.bytesMutex.Lock()
	defer transaction.bytesMutex.Unlock()

	// return bytes if the object has been marshaled in the mean time
	if bytes := transaction.bytes; bytes != nil {
		return bytes
	}

	// create marshal helper
	marshalUtil := marshalutil.New()

	// marshal essence bytes
	marshalUtil.WriteBytes(transaction.EssenceBytes())

	// marshal signature bytes
	marshalUtil.WriteBytes(transaction.SignatureBytes())

	// store marshaled result
	transaction.bytes = marshalUtil.Bytes()

	return transaction.bytes
}

func (transaction *Transaction) Sign(signature signaturescheme.SignatureScheme) *Transaction {
	transaction.signatures.Add(signature.Address(), signature.Sign(transaction.EssenceBytes()))

	return transaction
}

func (transaction *Transaction) String() string {
	id := transaction.Id()

	return stringify.Struct("Transaction"+fmt.Sprintf("(%p)", transaction),
		stringify.StructField("id", base58.Encode(id[:])),
		stringify.StructField("inputs", transaction.inputs),
		stringify.StructField("outputs", transaction.outputs),
		stringify.StructField("signatures", transaction.signatures),
		stringify.StructField("dataPayloadSize", transaction.DataPayloadSize()),
	)
}

// max dataPayload size limit
const MAX_DATA_PAYLOAD_SIZE = 64 * 1024

// SetDataPayload sets yhe dataPayload and its type
func (transaction *Transaction) SetDataPayload(data []byte) error {
	transaction.dataPayloadMutex.Lock()
	defer transaction.dataPayloadMutex.Unlock()

	if len(data) > MAX_DATA_PAYLOAD_SIZE {
		return fmt.Errorf("maximum dataPayload size of %d bytes exceeded", MAX_DATA_PAYLOAD_SIZE)
	}
	transaction.dataPayload = data
	return nil
}

// GetDataPayload gets the dataPayload and its type
func (transaction *Transaction) GetDataPayload() []byte {
	transaction.dataPayloadMutex.RLock()
	defer transaction.dataPayloadMutex.RUnlock()

	return transaction.dataPayload
}

// DataPayloadSize returns the size of the dataPayload as uint32.
// nil payload as size 0
func (transaction *Transaction) DataPayloadSize() uint32 {
	transaction.dataPayloadMutex.RLock()
	defer transaction.dataPayloadMutex.RUnlock()

	return uint32(len(transaction.dataPayload))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IMPLEMENT StorableObject interface ///////////////////////////////////////////////////////////////////////////

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Transaction{}

func (transaction *Transaction) ObjectStorageKey() []byte {
	return transaction.Id().Bytes()
}

func (transaction *Transaction) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// ObjectStorageValue returns a bytes representation of the Transaction by implementing the encoding.BinaryMarshaler interface.
func (transaction *Transaction) ObjectStorageValue() []byte {
	return transaction.Bytes()
}

func (transaction *Transaction) UnmarshalObjectStorageValue(bytes []byte) (err error, consumedBytes int) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// unmarshal inputs
	parsedInputs, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return InputsFromBytes(data) })
	if err != nil {
		return
	}
	transaction.inputs = parsedInputs.(*Inputs)

	// unmarshal outputs
	parsedOutputs, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return OutputsFromBytes(data) })
	if err != nil {
		return
	}
	transaction.outputs = parsedOutputs.(*Outputs)

	// unmarshal data payload size
	var dataPayloadSize uint32
	dataPayloadSize, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	if dataPayloadSize > MAX_DATA_PAYLOAD_SIZE {
		err = fmt.Errorf("data payload size of %d bytes exceeds maximum limit of %d bytes",
			dataPayloadSize, MAX_DATA_PAYLOAD_SIZE)
		return
	}

	// unmarshal data payload
	transaction.dataPayload, err = marshalUtil.ReadBytes(int(dataPayloadSize))
	if err != nil {
		return
	}

	// store essence bytes
	essenceBytesCount := marshalUtil.ReadOffset()
	transaction.essenceBytes = make([]byte, essenceBytesCount)
	copy(transaction.essenceBytes, bytes[:essenceBytesCount])

	// unmarshal outputs
	parsedSignatures, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return SignaturesFromBytes(data) })
	if err != nil {
		return
	}
	transaction.signatures = parsedSignatures.(*Signatures)

	// store signature bytes
	signatureBytesCount := marshalUtil.ReadOffset() - essenceBytesCount
	transaction.signatureBytes = make([]byte, signatureBytesCount)
	copy(transaction.signatureBytes, bytes[essenceBytesCount:essenceBytesCount+signatureBytesCount])

	// return the number of bytes we processed
	consumedBytes = essenceBytesCount + signatureBytesCount

	// store bytes, so we don't have to marshal manually
	transaction.bytes = bytes[:consumedBytes]

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// CachedTransaction is a wrapper for the object storage, that takes care of type casting the Transaction objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of Transaction, without having to manually type cast over and over again.
type CachedTransaction struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedTransaction instead of a generic CachedObject.
func (cachedTransaction *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{cachedTransaction.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedTransaction object instead of a generic CachedObject in the
// consumer).
func (cachedTransaction *CachedTransaction) Consume(consumer func(metadata *Transaction)) bool {
	return cachedTransaction.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Transaction))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
func (cachedTransaction *CachedTransaction) Unwrap() *Transaction {
	if untypedTransaction := cachedTransaction.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Transaction); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
