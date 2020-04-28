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

type Payload struct {
	objectstorage.StorableObjectFlags

	id      *Id
	idMutex sync.RWMutex

	trunkPayloadId  Id
	branchPayloadId Id
	transaction     *transaction.Transaction
	bytes           []byte
	bytesMutex      sync.RWMutex
}

func New(trunkPayloadId, branchPayloadId Id, valueTransfer *transaction.Transaction) *Payload {
	return &Payload{
		trunkPayloadId:  trunkPayloadId,
		branchPayloadId: branchPayloadId,
		transaction:     valueTransfer,
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func FromStorageKey(key []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
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
	if payloadId, idErr := ParseId(marshalUtil); idErr != nil {
		err = idErr

		return
	} else {
		result.id = &payloadId
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

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

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

func (payload *Payload) Id() Id {
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
	marshalUtil := marshalutil.New(IdLength + IdLength + transaction.IdLength)
	marshalUtil.WriteBytes(payload.trunkPayloadId.Bytes())
	marshalUtil.WriteBytes(payload.branchPayloadId.Bytes())
	marshalUtil.WriteBytes(payload.Transaction().Id().Bytes())

	var id Id = blake2b.Sum256(marshalUtil.Bytes())
	payload.id = &id

	return id
}

func (payload *Payload) TrunkId() Id {
	return payload.trunkPayloadId
}

func (payload *Payload) BranchId() Id {
	return payload.branchPayloadId
}

func (payload *Payload) Transaction() *transaction.Transaction {
	return payload.transaction
}

func (payload *Payload) Bytes() []byte {
	return payload.ObjectStorageValue()
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("id", payload.Id()),
		stringify.StructField("trunk", payload.TrunkId()),
		stringify.StructField("branch", payload.BranchId()),
		stringify.StructField("transfer", payload.Transaction()),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

var Type = payload.Type(1)

func (payload *Payload) Type() payload.Type {
	return Type
}

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
	payloadLength := IdLength + IdLength + len(transferBytes)
	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + marshalutil.UINT32_SIZE + payloadLength)
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteUint32(uint32(payloadLength))
	marshalUtil.WriteBytes(payload.trunkPayloadId.Bytes())
	marshalUtil.WriteBytes(payload.branchPayloadId.Bytes())
	marshalUtil.WriteBytes(transferBytes)
	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

func (payload *Payload) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
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
	if payload.trunkPayloadId, err = ParseId(marshalUtil); err != nil {
		return
	}
	if payload.branchPayloadId, err = ParseId(marshalUtil); err != nil {
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

func (payload *Payload) Unmarshal(data []byte) (err error) {
	_, err, _ = FromBytes(data)

	return
}

func init() {
	payload.RegisterType(Type, func(data []byte) (payload payload.Payload, err error) {
		payload, err, _ = FromBytes(data)

		return
	})
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorableObject implementation ////////////////////////////////////////////////////////////////////////////////

// ObjectStorageValue() (bytes []byte, err error) already implemented by Payload

// UnmarshalObjectStorageValue(data []byte) (err error) already implemented by Payload

func (payload *Payload) ObjectStorageKey() []byte {
	return payload.Id().Bytes()
}

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
	if untypedTransaction := cachedPayload.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Payload); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
