package transferoutput

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transfer"
)

// OutputId is the data type that represents the identifier for a Output.
type OutputId [OutputIdLength]byte

// NewOutputId is the constructor for the OutputId type.
func NewOutputId(outputAddress address.Address, transferId transfer.Id) (transferOutputId OutputId) {
	copy(transferOutputId[:address.Length], outputAddress.Bytes())
	copy(transferOutputId[address.Length:], transferId[:])

	return
}

// OutputIdFromBytes unmarshals an OutputId from a sequence of bytes.
func OutputIdFromBytes(bytes []byte) (result OutputId, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if idBytes, idErr := marshalUtil.ReadBytes(OutputIdLength); idErr != nil {
		err = idErr

		return
	} else {
		copy(result[:], idBytes)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling of Ids from a byte stream using the marshalUtil package.
func ParseOutputId(marshalUtil *marshalutil.MarshalUtil) (OutputId, error) {
	if transferMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return OutputIdFromBytes(data) }); err != nil {
		return OutputId{}, err
	} else {
		return transferMetadata.(OutputId), nil
	}
}

// Address returns the address part of an OutputId.
func (outputId OutputId) Address() (address address.Address) {
	copy(address[:], outputId[:])

	return
}

// TransferId returns the transfer id part of an OutputId.
func (outputId OutputId) TransferId() (transferId transfer.Id) {
	copy(transferId[:], outputId[address.Length:])

	return
}

// Bytes marshals the OutputId into a sequence of bytes.
func (outputId OutputId) Bytes() []byte {
	return outputId[:]
}

// String creates a human readable version of the OutputId (for debug purposes).
func (outputId OutputId) String() string {
	return base58.Encode(outputId[:])
}

// IdLength contains the amount of bytes that a marshaled version of the OutputId contains.
const OutputIdLength = address.Length + transfer.IdLength
