package payload

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

// Approver is a database entity, that allows us to keep track of the "tangle structure" by encoding which
// payload approves which other payload. It allows us to traverse the tangle in the opposite direction of the referenced
// trunk and branch payloads.
type Approver struct {
	objectstorage.StorableObjectFlags

	storageKey          []byte
	referencedPayloadId Id
	approvingPayloadId  Id
}

// NewApprover creates an approver object that encodes a single relation between an approved and an approving payload.
func NewApprover(referencedPayload Id, approvingPayload Id) *Approver {
	marshalUtil := marshalutil.New(IdLength + IdLength)
	marshalUtil.WriteBytes(referencedPayload.Bytes())
	marshalUtil.WriteBytes(approvingPayload.Bytes())

	return &Approver{
		referencedPayloadId: referencedPayload,
		approvingPayloadId:  approvingPayload,
		storageKey:          marshalUtil.Bytes(),
	}
}

// ApproverFromStorage get's called when we restore transaction metadata from the storage.
// In contrast to other database models, it unmarshals the information from the key and does not use the UnmarshalBinary
// method.
func ApproverFromStorage(idBytes []byte) objectstorage.StorableObject {
	marshalUtil := marshalutil.New(idBytes)

	referencedPayloadId, err := ParseId(marshalUtil)
	if err != nil {
		panic(err)
	}
	approvingPayloadId, err := ParseId(marshalUtil)
	if err != nil {
		panic(err)
	}

	result := &Approver{
		referencedPayloadId: referencedPayloadId,
		approvingPayloadId:  approvingPayloadId,
		storageKey:          marshalUtil.Bytes(true),
	}

	return result
}

// GetApprovingPayloadId returns the id of the approving payload.
func (approver *Approver) GetApprovingPayloadId() Id {
	return approver.approvingPayloadId
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (approver *Approver) GetStorageKey() []byte {
	return approver.storageKey
}

// MarshalBinary is implemented to conform with the StorableObject interface, but it does not really do anything,
// since all of the information about an approver are stored in the "key".
func (approver *Approver) MarshalBinary() (data []byte, err error) {
	return
}

// UnmarshalBinary is implemented to conform with the StorableObject interface, but it does not really do anything,
// since all of the information about an approver are stored in the "key".
func (approver *Approver) UnmarshalBinary(data []byte) error {
	return nil
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (approver *Approver) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// CachedApprover is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedApprovers, without having to manually type cast over and over again.
type CachedApprover struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedApprover *CachedApprover) Retain() *CachedApprover {
	return &CachedApprover{cachedApprover.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedApprover *CachedApprover) Consume(consumer func(payload *Approver)) bool {
	return cachedApprover.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Approver))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedApprover *CachedApprover) Unwrap() *Approver {
	if untypedTransaction := cachedApprover.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Approver); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}

type CachedApprovers []*CachedApprover

func (cachedApprovers CachedApprovers) Consume(consumer func(approver *Approver)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = cachedApprover.Consume(consumer) || consumed
	}

	return
}
