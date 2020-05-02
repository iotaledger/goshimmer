package payload

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

// Payload represents the entity that forms the Tangle by referencing other Payloads using their trunk and branch.
// A Payload contains a transaction and defines, where in the Tangle a transaction is attached.
type Payload struct {
	objectstorage.StorableObjectFlags

	id      *ID
	idMutex sync.RWMutex

	trunkPayloadID  ID
	branchPayloadID ID
	transaction     *transaction.Transaction
	bytes           []byte
	bytesMutex      sync.RWMutex
}

// New is the constructor of a Payload and creates a new Payload object from the given details.
func New(trunkPayloadID, branchPayloadID ID, valueTransfer *transaction.Transaction) *Payload {
	return &Payload{
		trunkPayloadID:  trunkPayloadID,
		branchPayloadID: branchPayloadID,
		transaction:     valueTransfer,
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromStorageKey is a factory method that creates a new Payload instance from a storage key of the objectstorage.
// It is used by the objectstorage, to create new instances of this entity.
func FromStorageKey(key []byte, optionalTargetObject ...*Payload) (result *Payload, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingPayloadFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	payloadID, idErr := ParseID(marshalUtil)
	if idErr != nil {
		err = idErr

		return
	}
	result.id = &payloadID
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse unmarshals a Payload using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Payload) (result *Payload, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
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
	marshalUtil.WriteBytes(payload.trunkPayloadID.Bytes())
	marshalUtil.WriteBytes(payload.branchPayloadID.Bytes())
	marshalUtil.WriteBytes(payload.Transaction().ID().Bytes())

	var id ID = blake2b.Sum256(marshalUtil.Bytes())
	payload.id = &id

	return id
}

// TrunkID returns the first Payload that is referenced by this Payload.
func (payload *Payload) TrunkID() ID {
	return payload.trunkPayloadID
}

// BranchID returns the second Payload that is referenced by this Payload.
func (payload *Payload) BranchID() ID {
	return payload.branchPayloadID
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
		stringify.StructField("trunk", payload.TrunkID()),
		stringify.StructField("branch", payload.BranchID()),
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
// Branch.
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
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteUint32(uint32(payloadLength))
	marshalUtil.WriteBytes(payload.trunkPayloadID.Bytes())
	marshalUtil.WriteBytes(payload.branchPayloadID.Bytes())
	marshalUtil.WriteBytes(transferBytes)
	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

// UnmarshalObjectStorageValue unmarshals the bytes that are stored in the value of the objectstorage.
func (payload *Payload) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)

	// read information that are required to identify the payload from the outside
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	_, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	// parse trunk payload id
	if payload.trunkPayloadID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if payload.branchPayloadID, err = ParseID(marshalUtil); err != nil {
		return
	}
	if payload.transaction, err = transaction.Parse(marshalUtil); err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	payload.bytes = make([]byte, consumedBytes)
	copy(payload.bytes, data[:consumedBytes])

	return
}

// Unmarshal unmarshals a given slice of bytes and fills the object with the.
func (payload *Payload) Unmarshal(data []byte) (err error) {
	_, _, err = FromBytes(data, payload)

	return
}

func init() {
	payload.RegisterType(Type, func(data []byte) (payload payload.Payload, err error) {
		payload, _, err = FromBytes(data)

		return
	})
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorableObject implementation ////////////////////////////////////////////////////////////////////////////////

// ObjectStorageKey returns the bytes that are used a key when storing the Branch in an objectstorage.
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
