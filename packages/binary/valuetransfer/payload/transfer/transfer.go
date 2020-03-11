package transfer

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/inputs"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/outputs"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/signatures"
)

// region IMPLEMENT Transfer ///////////////////////////////////////////////////////////////////////////////////////////

type Transfer struct {
	objectstorage.StorableObjectFlags

	inputs     *inputs.Inputs
	outputs    *outputs.Outputs
	signatures *signatures.Signatures

	id      *id.Id
	idMutex sync.RWMutex

	essenceBytes      []byte
	essenceBytesMutex sync.RWMutex

	signatureBytes      []byte
	signatureBytesMutex sync.RWMutex

	bytes      []byte
	bytesMutex sync.RWMutex
}

func New(inputs *inputs.Inputs, outputs *outputs.Outputs) *Transfer {
	return &Transfer{
		inputs:     inputs,
		outputs:    outputs,
		signatures: signatures.New(),
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

	// store essence bytes
	essenceBytesCount := marshalUtil.ReadOffset()
	result.essenceBytes = make([]byte, essenceBytesCount)
	copy(result.essenceBytes, bytes[:essenceBytesCount])

	// unmarshal outputs
	parsedSignatures, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return signatures.FromBytes(data) })
	if err != nil {
		return
	}
	result.signatures = parsedSignatures.(*signatures.Signatures)

	// store signature bytes
	signatureBytesCount := marshalUtil.ReadOffset() - essenceBytesCount
	result.signatureBytes = make([]byte, signatureBytesCount)
	copy(result.signatureBytes, bytes[essenceBytesCount:essenceBytesCount+signatureBytesCount])

	// return the number of bytes we processed
	consumedBytes = essenceBytesCount + signatureBytesCount

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

func FromStorage(key []byte) *Transfer {
	transferId := id.New(key)

	return &Transfer{
		id: &transferId,
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

func (transfer *Transfer) SignaturesValid() bool {
	signaturesValid := true
	transfer.inputs.ForEachAddress(func(address address.Address) bool {
		if signature, exists := transfer.signatures.Get(address); !exists || !signature.IsValid(transfer.EssenceBytes()) {
			signaturesValid = false

			return false
		}

		return true
	})

	return signaturesValid
}

func (transfer *Transfer) EssenceBytes() []byte {
	// acquire read lock on essenceBytes
	transfer.essenceBytesMutex.RLock()

	// return essenceBytes if the object has been marshaled already
	if transfer.essenceBytes != nil {
		defer transfer.essenceBytesMutex.RUnlock()

		return transfer.essenceBytes
	}

	// switch to write lock
	transfer.essenceBytesMutex.RUnlock()
	transfer.essenceBytesMutex.Lock()
	defer transfer.essenceBytesMutex.Unlock()

	// return essenceBytes if the object has been marshaled in the mean time
	if essenceBytes := transfer.essenceBytes; essenceBytes != nil {
		return essenceBytes
	}

	// create marshal helper
	marshalUtil := marshalutil.New()

	// marshal inputs
	marshalUtil.WriteBytes(transfer.inputs.Bytes())

	// marshal outputs
	marshalUtil.WriteBytes(transfer.outputs.Bytes())

	// store marshaled result
	transfer.essenceBytes = marshalUtil.Bytes()

	return transfer.essenceBytes
}

func (transfer *Transfer) SignatureBytes() []byte {
	transfer.signatureBytesMutex.RLock()
	if transfer.signatureBytes != nil {
		defer transfer.signatureBytesMutex.RUnlock()

		return transfer.signatureBytes
	}

	transfer.signatureBytesMutex.RUnlock()
	transfer.signatureBytesMutex.Lock()
	defer transfer.signatureBytesMutex.Unlock()

	if transfer.signatureBytes != nil {
		return transfer.signatureBytes
	}

	// generate signatures
	transfer.signatureBytes = transfer.signatures.Bytes()

	return transfer.signatureBytes
}

func (transfer *Transfer) Bytes() []byte {
	// acquire read lock on bytes
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

	// marshal essence bytes
	marshalUtil.WriteBytes(transfer.EssenceBytes())

	// marshal signature bytes
	marshalUtil.WriteBytes(transfer.SignatureBytes())

	// store marshaled result
	transfer.bytes = marshalUtil.Bytes()

	return transfer.bytes
}

func (transfer *Transfer) Sign(signature signatures.SignatureScheme) *Transfer {
	transfer.signatures.Add(signature.Address(), signature.Sign(transfer.EssenceBytes()))

	return transfer
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
