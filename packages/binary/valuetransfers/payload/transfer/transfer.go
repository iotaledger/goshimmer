package transfer

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/inputs"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/outputs"
)

// region IMPLEMENT Transfer ///////////////////////////////////////////////////////////////////////////////////////////

type Transfer struct {
	objectstorage.StorableObjectFlags

	id      *id.Id
	inputs  *inputs.Inputs
	outputs *outputs.Outputs
	bytes   []byte

	idMutex    sync.RWMutex
	bytesMutex sync.RWMutex
}

func New(inputs *inputs.Inputs, outputs *outputs.Outputs) *Transfer {
	return &Transfer{
		inputs:  inputs,
		outputs: outputs,
	}
}

func FromBytes(bytes []byte, optionalTargetObject ...*Transfer) (result *Transfer, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Transfer{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// unmarshal inputs
	parsedInputs, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return inputs.FromBytes(data) })
	if err != nil {
		return
	}
	result.inputs = parsedInputs.(*inputs.Inputs)

	// unmarshal outputs
	parsedOutputs, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return outputs.FromBytes(data) })
	if err != nil {
		return
	}
	result.outputs = parsedOutputs.(*outputs.Outputs)

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

func TransferFromStorage(key []byte) *Transfer {
	id := id.New(key)

	return &Transfer{
		id: &id,
	}
}

func (transfer *Transfer) GetId() id.Id {
	// acquire lock for reading id
	transfer.idMutex.RLock()

	// return if id has been calculated already
	if transfer.id != nil {
		defer transfer.idMutex.RUnlock()

		return *transfer.id
	}

	// switch to write lock
	transfer.idMutex.RUnlock()
	transfer.idMutex.Lock()
	defer transfer.idMutex.Unlock()

	// return if id has been calculated in the mean time
	if transfer.id != nil {
		return *transfer.id
	}

	// otherwise calculate the id
	idBytes := blake2b.Sum256(transfer.Bytes())
	transferId := id.New(idBytes[:])

	// cache result for later calls
	transfer.id = &transferId

	return transferId
}

func (transfer *Transfer) Bytes() []byte {
	// acquired read lock on bytes
	transfer.bytesMutex.RLock()

	// return bytes if the object has been marshaled already
	if transfer.bytes != nil {
		defer transfer.bytesMutex.RUnlock()

		return transfer.bytes
	}

	// switch to write lock
	transfer.bytesMutex.RUnlock()
	transfer.bytesMutex.Lock()
	defer transfer.bytesMutex.Unlock()

	// return bytes if the object has been marshaled in the mean time
	if bytes := transfer.bytes; bytes != nil {
		return bytes
	}

	// create marshal helper
	marshalUtil := marshalutil.New()

	// marshal inputs
	marshalUtil.WriteBytes(transfer.inputs.ToBytes())

	// marshal outputs
	marshalUtil.WriteBytes(transfer.outputs.Bytes())

	// store marshaled result
	transfer.bytes = marshalUtil.Bytes()

	return transfer.bytes
}

func (transfer *Transfer) String() string {
	id := transfer.GetId()

	return stringify.Struct("Transfer"+fmt.Sprintf("(%p)", transfer),
		stringify.StructField("id", base58.Encode(id[:])),
		stringify.StructField("inputs", transfer.inputs),
		stringify.StructField("outputs", transfer.outputs),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IMPLEMENT StorableObject interface ///////////////////////////////////////////////////////////////////////////

// define contract (ensure that the struct fulfills the given interface)
var _ objectstorage.StorableObject = &Transfer{}

func (transfer *Transfer) GetStorageKey() []byte {
	id := transfer.GetId()

	return id[:]
}

func (transfer *Transfer) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary returns a bytes representation of the transfer by implementing the encoding.BinaryMarshaler interface.
func (transfer *Transfer) MarshalBinary() ([]byte, error) {
	return transfer.Bytes(), nil
}

func (transfer *Transfer) UnmarshalBinary(bytes []byte) (err error) {
	_, err, _ = FromBytes(bytes, transfer)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
