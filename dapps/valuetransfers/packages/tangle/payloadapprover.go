package tangle

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

// PayloadApprover is a database entity, that allows us to keep track of the "tangle structure" by encoding which
// payload approves which other payload. It allows us to traverse the tangle in the opposite direction of the referenced
// trunk and branch payloads.
type PayloadApprover struct {
	objectstorage.StorableObjectFlags

	storageKey          []byte
	referencedPayloadID payload.ID
	approvingPayloadID  payload.ID
}

// NewPayloadApprover creates an approver object that encodes a single relation between an approved and an approving payload.
func NewPayloadApprover(referencedPayload payload.ID, approvingPayload payload.ID) *PayloadApprover {
	marshalUtil := marshalutil.New(payload.IDLength + payload.IDLength)
	marshalUtil.WriteBytes(referencedPayload.Bytes())
	marshalUtil.WriteBytes(approvingPayload.Bytes())

	return &PayloadApprover{
		referencedPayloadID: referencedPayload,
		approvingPayloadID:  approvingPayload,
		storageKey:          marshalUtil.Bytes(),
	}
}

// PayloadApproverFromBytes unmarshals a PayloadApprover from a sequence of bytes.
func PayloadApproverFromBytes(bytes []byte, optionalTargetObject ...*PayloadApprover) (result *PayloadApprover, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParsePayloadApprover(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParsePayloadApprover unmarshals a PayloadApprover using the given marshalUtil (for easier marshaling/unmarshaling).
func ParsePayloadApprover(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*PayloadApprover) (result *PayloadApprover, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return PayloadApproverFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*PayloadApprover)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// PayloadApproverFromStorageKey get's called when we restore transaction metadata from the storage.
// In contrast to other database models, it unmarshals the information from the key and does not use the UnmarshalObjectStorageValue
// method.
func PayloadApproverFromStorageKey(key []byte, optionalTargetObject ...*PayloadApprover) (result *PayloadApprover, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &PayloadApprover{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to PayloadApproverFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.referencedPayloadID, err = payload.ParseID(marshalUtil); err != nil {
		return
	}
	if result.approvingPayloadID, err = payload.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	result.storageKey = marshalutil.New(key[:consumedBytes]).Bytes(true)

	return
}

// ApprovingPayloadID returns the identifier of the approving payload.
func (payloadApprover *PayloadApprover) ApprovingPayloadID() payload.ID {
	return payloadApprover.approvingPayloadID
}

// ObjectStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (payloadApprover *PayloadApprover) ObjectStorageKey() []byte {
	return payloadApprover.storageKey
}

// ObjectStorageValue is implemented to conform with the StorableObject interface, but it does not really do anything,
// since all of the information about an approver are stored in the "key".
func (payloadApprover *PayloadApprover) ObjectStorageValue() (data []byte) {
	return
}

// UnmarshalObjectStorageValue is implemented to conform with the StorableObject interface, but it does not really do
// anything, since all of the information about an approver are stored in the "key".
func (payloadApprover *PayloadApprover) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	return
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (payloadApprover *PayloadApprover) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// CachedPayloadApprover is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedApprovers, without having to manually type cast over and over again.
type CachedPayloadApprover struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedPayloadApprover *CachedPayloadApprover) Retain() *CachedPayloadApprover {
	return &CachedPayloadApprover{cachedPayloadApprover.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedPayloadApprover *CachedPayloadApprover) Consume(consumer func(payload *PayloadApprover)) bool {
	return cachedPayloadApprover.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*PayloadApprover))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedPayloadApprover *CachedPayloadApprover) Unwrap() *PayloadApprover {
	untypedTransaction := cachedPayloadApprover.Get()
	if untypedTransaction == nil {
		return nil
	}

	typeCastedTransaction := untypedTransaction.(*PayloadApprover)
	if typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
		return nil
	}

	return typeCastedTransaction
}

// CachedApprovers represents a collection of CachedPayloadApprover.
type CachedApprovers []*CachedPayloadApprover

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedApprovers CachedApprovers) Consume(consumer func(approver *PayloadApprover)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = cachedApprover.Consume(consumer) || consumed
	}

	return
}
