package payloadapprover

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
)

// PayloadApprover is a database entity, that allows us to keep track of the "tangle structure" by encoding which
// payload approves which other payload. It allows us to traverse the tangle in the opposite direction of the referenced
// trunk and branch payloads.
type PayloadApprover struct {
	objectstorage.StorableObjectFlags

	storageKey          []byte
	referencedPayloadId payloadid.Id
	approvingPayloadId  payloadid.Id
}

// New creates an approver object that encodes a single relation between an approved and an approving payload.
func New(referencedPayload payloadid.Id, approvingPayload payloadid.Id) *PayloadApprover {
	marshalUtil := marshalutil.New(payloadid.Length + payloadid.Length)
	marshalUtil.WriteBytes(referencedPayload.Bytes())
	marshalUtil.WriteBytes(approvingPayload.Bytes())

	return &PayloadApprover{
		referencedPayloadId: referencedPayload,
		approvingPayloadId:  approvingPayload,
		storageKey:          marshalUtil.Bytes(),
	}
}

// FromStorage get's called when we restore transaction metadata from the storage.
// In contrast to other database models, it unmarshals the information from the key and does not use the UnmarshalBinary
// method.
func FromStorage(idBytes []byte) objectstorage.StorableObject {
	marshalUtil := marshalutil.New(idBytes)

	referencedPayloadId, err := payloadid.Parse(marshalUtil)
	if err != nil {
		panic(err)
	}
	approvingPayloadId, err := payloadid.Parse(marshalUtil)
	if err != nil {
		panic(err)
	}

	result := &PayloadApprover{
		referencedPayloadId: referencedPayloadId,
		approvingPayloadId:  approvingPayloadId,
		storageKey:          marshalUtil.Bytes(true),
	}

	return result
}

// GetApprovingPayloadId returns the id of the approving payload.
func (approver *PayloadApprover) GetApprovingPayloadId() payloadid.Id {
	return approver.approvingPayloadId
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (approver *PayloadApprover) GetStorageKey() []byte {
	return approver.storageKey
}

// MarshalBinary is implemented to conform with the StorableObject interface, but it does not really do anything,
// since all of the information about an approver are stored in the "key".
func (approver *PayloadApprover) MarshalBinary() (data []byte, err error) {
	return
}

// UnmarshalBinary is implemented to conform with the StorableObject interface, but it does not really do anything,
// since all of the information about an approver are stored in the "key".
func (approver *PayloadApprover) UnmarshalBinary(data []byte) error {
	return nil
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (approver *PayloadApprover) Update(other objectstorage.StorableObject) {
	panic("implement me")
}
