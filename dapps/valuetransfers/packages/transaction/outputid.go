package transaction

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// OutputId is the data type that represents the identifier for a Output.
type OutputId [OutputIdLength]byte

// NewOutputId is the constructor for the OutputId type.
func NewOutputId(outputAddress address.Address, transactionId Id) (outputId OutputId) {
	copy(outputId[:address.Length], outputAddress.Bytes())
	copy(outputId[address.Length:], transactionId[:])

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
	if outputId, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return OutputIdFromBytes(data) }); err != nil {
		return OutputId{}, err
	} else {
		return outputId.(OutputId), nil
	}
}

// Address returns the address part of an OutputId.
func (outputId OutputId) Address() (address address.Address) {
	copy(address[:], outputId[:])

	return
}

// TransactionId returns the transaction id part of an OutputId.
func (outputId OutputId) TransactionId() (transactionId Id) {
	copy(transactionId[:], outputId[address.Length:])

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
const OutputIdLength = address.Length + IdLength
