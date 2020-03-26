package tangle

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
)

// PayloadApprover is a database entity, that allows us to keep track of the "tangle structure" by encoding which
// payload approves which other payload. It allows us to traverse the tangle in the opposite direction of the referenced
// trunk and branch payloads.
type PayloadApprover struct {
	objectstorage.StorableObjectFlags

	storageKey          []byte
	referencedPayloadId payload.Id
	approvingPayloadId  payload.Id
}

// NewPayloadApprover creates an approver object that encodes a single relation between an approved and an approving payload.
func NewPayloadApprover(referencedPayload payload.Id, approvingPayload payload.Id) *PayloadApprover {
	marshalUtil := marshalutil.New(payload.IdLength + payload.IdLength)
	marshalUtil.WriteBytes(referencedPayload.Bytes())
	marshalUtil.WriteBytes(approvingPayload.Bytes())

	return &PayloadApprover{
		referencedPayloadId: referencedPayload,
		approvingPayloadId:  approvingPayload,
		storageKey:          marshalUtil.Bytes(),
	}
}

// PayloadApproverFromStorageKey get's called when we restore transaction metadata from the storage.
// In contrast to other database models, it unmarshals the information from the key and does not use the UnmarshalObjectStorageValue
// method.
func PayloadApproverFromStorageKey(idBytes []byte) (objectstorage.StorableObject, error) {
	marshalUtil := marshalutil.New(idBytes)

	referencedPayloadId, err := payload.ParseId(marshalUtil)
	if err != nil {
		return nil, err
	}
	approvingPayloadId, err := payload.ParseId(marshalUtil)
	if err != nil {
		return nil, err
	}

	result := &PayloadApprover{
		referencedPayloadId: referencedPayloadId,
		approvingPayloadId:  approvingPayloadId,
		storageKey:          marshalUtil.Bytes(true),
	}

	return result, nil
}

// GetApprovingPayloadId returns the id of the approving payload.
func (payloadApprover *PayloadApprover) GetApprovingPayloadId() payload.Id {
	return payloadApprover.approvingPayloadId
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
func (payloadApprover *PayloadApprover) UnmarshalObjectStorageValue(data []byte) error {
	return nil
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
	if untypedTransaction := cachedPayloadApprover.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*PayloadApprover); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}

type CachedApprovers []*CachedPayloadApprover

func (cachedApprovers CachedApprovers) Consume(consumer func(approver *PayloadApprover)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = cachedApprover.Consume(consumer) || consumed
	}

	return
}
