package payload

import (
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer"
	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
)

type Payload struct {
	objectstorage.StorableObjectFlags

	id              *payloadid.Id
	trunkPayloadId  payloadid.Id
	branchPayloadId payloadid.Id
	transfer        *transfer.Transfer
	bytes           []byte

	idMutex    sync.RWMutex
	bytesMutex sync.RWMutex
}

func NewPayload(trunkPayloadId, branchPayloadId payloadid.Id, valueTransfer *transfer.Transfer) *Payload {
	return &Payload{
		trunkPayloadId:  trunkPayloadId,
		branchPayloadId: branchPayloadId,
		transfer:        valueTransfer,
	}
}

func (payload *Payload) GetId() payloadid.Id {
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
	transferId := payload.GetTransfer().GetId()
	marshalUtil := marshalutil.New(payloadid.Length + payloadid.Length + transferid.Length)
	marshalUtil.WriteBytes(payload.trunkPayloadId[:])
	marshalUtil.WriteBytes(payload.branchPayloadId[:])
	marshalUtil.WriteBytes(transferId[:])
	var id payloadid.Id = blake2b.Sum256(marshalUtil.Bytes())
	payload.id = &id

	return id
}

func (payload *Payload) GetTrunkPayloadId() payloadid.Id {
	return payload.trunkPayloadId
}

func (payload *Payload) GetBranchPayloadId() payloadid.Id {
	return payload.branchPayloadId
}

func (payload *Payload) GetTransfer() *transfer.Transfer {
	return payload.transfer
}

func (payload *Payload) Bytes() (bytes []byte) {
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
	transferBytes, err := payload.GetTransfer().MarshalBinary()
	if err != nil {
		return
	}

	// marshal fields
	marshalUtil := marshalutil.New(payloadid.Length + payloadid.Length + transferid.Length)
	marshalUtil.WriteBytes(payload.trunkPayloadId[:])
	marshalUtil.WriteBytes(payload.branchPayloadId[:])
	marshalUtil.WriteBytes(transferBytes)
	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("id", payload.GetId()),
		stringify.StructField("trunk", payload.GetTrunkPayloadId()),
		stringify.StructField("branch", payload.GetBranchPayloadId()),
		stringify.StructField("transfer", payload.GetTransfer()),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

var Type = payload.Type(1)

func (payload *Payload) GetType() payload.Type {
	return Type
}

func (payload *Payload) MarshalBinary() (bytes []byte, err error) {
	return payload.Bytes(), nil
}

func (payload *Payload) UnmarshalBinary(data []byte) (err error) {
	marshalUtil := marshalutil.New(data)

	trunkTransactionIdBytes, err := marshalUtil.ReadBytes(payloadid.Length)
	if err != nil {
		return
	}

	branchTransactionIdBytes, err := marshalUtil.ReadBytes(payloadid.Length)
	if err != nil {
		return
	}

	valueTransfer := &transfer.Transfer{}
	if err = valueTransfer.UnmarshalBinary(marshalUtil.ReadRemainingBytes()); err != nil {
		return
	}

	payload.trunkPayloadId = payloadid.New(trunkTransactionIdBytes)
	payload.branchPayloadId = payloadid.New(branchTransactionIdBytes)
	payload.transfer = valueTransfer
	payload.bytes = data

	return
}

func init() {
	payload.RegisterType(Type, func(data []byte) (payload payload.Payload, err error) {
		payload = &Payload{}
		err = payload.UnmarshalBinary(data)

		return
	})
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorableObject implementation ////////////////////////////////////////////////////////////////////////////////

// MarshalBinary() (bytes []byte, err error) already implemented by Payload

// UnmarshalBinary(data []byte) (err error) already implemented by Payload

func (payload *Payload) GetStorageKey() []byte {
	id := payload.GetId()

	return id[:]
}

func (payload *Payload) Update(other objectstorage.StorableObject) {
	panic("a Payload should never be updated")
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ objectstorage.StorableObject = &Payload{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
