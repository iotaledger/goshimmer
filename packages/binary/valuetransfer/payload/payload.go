package payload

import (
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer"
	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

type Payload struct {
	objectstorage.StorableObjectFlags

	trunkPayloadId  payloadid.Id
	branchPayloadId payloadid.Id
	transfer        *transfer.Transfer

	id      *payloadid.Id
	idMutex sync.RWMutex

	bytes      []byte
	bytesMutex sync.RWMutex
}

func New(trunkPayloadId, branchPayloadId payloadid.Id, valueTransfer *transfer.Transfer) *Payload {
	return &Payload{
		trunkPayloadId:  trunkPayloadId,
		branchPayloadId: branchPayloadId,
		transfer:        valueTransfer,
	}
}

func FromStorage(key []byte) objectstorage.StorableObject {
	id, err, _ := payloadid.FromBytes(key)
	if err != nil {
		panic(err)
	}

	return &Payload{
		id: &id,
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

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
	parsedTrunkPayloadId, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return payloadid.FromBytes(data) })
	if err != nil {
		return
	}
	result.trunkPayloadId = parsedTrunkPayloadId.(payloadid.Id)

	// parse branch payload id
	parsedBranchPayloadId, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return payloadid.FromBytes(data) })
	if err != nil {
		return
	}
	result.branchPayloadId = parsedBranchPayloadId.(payloadid.Id)

	// parse transfer
	parsedTransfer, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return transfer.FromBytes(data) })
	if err != nil {
		return
	}
	result.transfer = parsedTransfer.(*transfer.Transfer)

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
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
	marshalUtil := marshalutil.New(payloadid.Length + payloadid.Length + transferid.Length)
	marshalUtil.WriteBytes(payload.trunkPayloadId.Bytes())
	marshalUtil.WriteBytes(payload.branchPayloadId.Bytes())
	marshalUtil.WriteBytes(payload.GetTransfer().GetId().Bytes())

	var id payloadid.Id = blake2b.Sum256(marshalUtil.Bytes())
	payload.id = &id

	return id
}

func (payload *Payload) GetTrunkId() payloadid.Id {
	return payload.trunkPayloadId
}

func (payload *Payload) GetBranchId() payloadid.Id {
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
	payloadLength := payloadid.Length + payloadid.Length + len(transferBytes)
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

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("id", payload.GetId()),
		stringify.StructField("trunk", payload.GetTrunkId()),
		stringify.StructField("branch", payload.GetBranchId()),
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
	_, err, _ = FromBytes(data, payload)

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
