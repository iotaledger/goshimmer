package payload

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

const (
	// ObjectName defines the name of the value object.
	ObjectName = "value"
)

// Payload represents the entity that forms the Tangle by referencing other Payloads using their parent1 and parent2.
// A Payload contains a transaction and defines, where in the Tangle a transaction is attached.
type Payload struct {
	objectstorage.StorableObjectFlags

	id      *ID
	idMutex sync.RWMutex

	parent1PayloadID ID
	parent2PayloadID ID
	transaction      *transaction.Transaction
	bytes            []byte
	bytesMutex       sync.RWMutex
}

// New is the constructor of a Payload and creates a new Payload object from the given details.
func New(parent1PayloadID, parent2PayloadID ID, valueTransfer *transaction.Transaction) *Payload {
	return &Payload{
		parent1PayloadID: parent1PayloadID,
		parent2PayloadID: parent2PayloadID,
		transaction:      valueTransfer,
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromObjectStorage is a factory method that creates a new Payload from the ObjectStorage.
func FromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	// parse the message
	parsedPayload, err := Parse(marshalutil.New(data))
	if err != nil {
		err = fmt.Errorf("failed to parse value payload from object storage: %w", err)
		return
	}

	// parse the ID from the key
	payloadID, err := ParseID(marshalutil.New(key))
	if err != nil {
		err = fmt.Errorf("failed to parse value payload ID from object storage: %w", err)
		return
	}
	parsedPayload.id = &payloadID

	// assign result
	result = parsedPayload

	return
}

// Parse unmarshals a Payload using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Payload, err error) {
	// determine read offset before starting to parse
	readOffsetStart := marshalUtil.ReadOffset()

	// read information that are required to identify the payload from the outside
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse payload size of value payload: %w", err)
		return
	}
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse payload type of value payload: %w", err)
		return
	}

	// parse parent1 payload id
	result = &Payload{}
	if result.parent1PayloadID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if result.parent2PayloadID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if result.transaction, err = transaction.Parse(marshalUtil); err != nil {
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

// ID returns the identifier if the Payload.
func (payload *Payload) ID() ID {
	// acquire lock for reading id
	payload.idMutex.RLock()

	// return if id has been calculated already
	if payload.id != nil {
		defer payload.idMutex.RUnlock()

		return *payload.id
	}

	// switch to write lock
	payload.idMutex.RUnlock()
	payload.idMutex.Lock()
	defer payload.idMutex.Unlock()

	// return if id has been calculated in the mean time
	if payload.id != nil {
		return *payload.id
	}

	// otherwise calculate the id
	marshalUtil := marshalutil.New(IDLength + IDLength + transaction.IDLength)
	marshalUtil.WriteBytes(payload.parent1PayloadID.Bytes())
	marshalUtil.WriteBytes(payload.parent2PayloadID.Bytes())
	marshalUtil.WriteBytes(payload.Transaction().ID().Bytes())

	var id ID = blake2b.Sum256(marshalUtil.Bytes())
	payload.id = &id

	return id
}

// Parent1ID returns the first Payload that is referenced by this Payload.
func (payload *Payload) Parent1ID() ID {
	return payload.parent1PayloadID
}

// Parent2ID returns the second Payload that is referenced by this Payload.
func (payload *Payload) Parent2ID() ID {
	return payload.parent2PayloadID
}

// Transaction returns the Transaction that is being attached in this Payload.
func (payload *Payload) Transaction() *transaction.Transaction {
	return payload.transaction
}

// Bytes returns a marshaled version of this Payload.
func (payload *Payload) Bytes() []byte {
	return payload.ObjectStorageValue()
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("id", payload.ID()),
		stringify.StructField("parent1", payload.Parent1ID()),
		stringify.StructField("parent2", payload.Parent2ID()),
		stringify.StructField("transfer", payload.Transaction()),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the value Payload type.
const Type = payload.Type(1)

// Type returns the type of the Payload.
func (payload *Payload) Type() payload.Type {
	return Type
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// Parent2.
func (payload *Payload) ObjectStorageValue() (bytes []byte) {
	// acquire lock for reading bytes
	payload.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = payload.bytes; bytes != nil {
		defer payload.bytesMutex.RUnlock()

		return
	}

	// switch to write lock
	payload.bytesMutex.RUnlock()
	payload.bytesMutex.Lock()
	defer payload.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = payload.bytes; bytes != nil {
		return
	}

	// retrieve bytes of transfer
	transferBytes := payload.Transaction().ObjectStorageValue()

	// marshal fields
	payloadLength := IDLength + IDLength + len(transferBytes)
	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + marshalutil.UINT32_SIZE + payloadLength)
	marshalUtil.WriteUint32(uint32(payloadLength))
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteBytes(payload.parent1PayloadID.Bytes())
	marshalUtil.WriteBytes(payload.parent2PayloadID.Bytes())
	marshalUtil.WriteBytes(transferBytes)
	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

func init() {
	payload.RegisterType(Type, ObjectName, func(data []byte) (payload payload.Payload, err error) {
		payload, _, err = FromBytes(data)

		return
	})
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorableObject implementation ////////////////////////////////////////////////////////////////////////////////

// ObjectStorageKey returns the bytes that are used a key when storing the  Payload in an objectstorage.
func (payload *Payload) ObjectStorageKey() []byte {
	return payload.ID().Bytes()
}

// Update is disabled but needs to be implemented to be compatible with the objectstorage.
func (payload *Payload) Update(other objectstorage.StorableObject) {
	panic("a Payload should never be updated")
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ objectstorage.StorableObject = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// CachedPayload is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedObjects, without having to manually type cast over and over again.
type CachedPayload struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedPayload *CachedPayload) Retain() *CachedPayload {
	return &CachedPayload{cachedPayload.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedPayload *CachedPayload) Consume(consumer func(payload *Payload)) bool {
	return cachedPayload.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Payload))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedPayload *CachedPayload) Unwrap() *Payload {
	untypedTransaction := cachedPayload.Get()
	if untypedTransaction == nil {
		return nil
	}

	typeCastedTransaction := untypedTransaction.(*Payload)
	if typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
		return nil
	}

	return typeCastedTransaction
}
