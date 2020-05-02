package transaction

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
)

// OutputID is the data type that represents the identifier for a Output.
type OutputID [OutputIDLength]byte

// NewOutputID is the constructor for the OutputID type.
func NewOutputID(outputAddress address.Address, transactionID ID) (outputID OutputID) {
	copy(outputID[:address.Length], outputAddress.Bytes())
	copy(outputID[address.Length:], transactionID[:])

	return
}

// OutputIDFromBytes unmarshals an OutputID from a sequence of bytes.
func OutputIDFromBytes(bytes []byte) (result OutputID, consumedBytes int, err error) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	idBytes, idErr := marshalUtil.ReadBytes(OutputIDLength)
	if idErr != nil {
		err = idErr

		return
	}
	copy(result[:], idBytes)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseOutputID is a wrapper for simplified unmarshaling of Ids from a byte stream using the marshalUtil package.
func ParseOutputID(marshalUtil *marshalutil.MarshalUtil) (OutputID, error) {
	outputID, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return OutputIDFromBytes(data) })
	if err != nil {
		return OutputID{}, err
	}

	return outputID.(OutputID), nil
}

// Address returns the address part of an OutputID.
func (outputID OutputID) Address() (address address.Address) {
	copy(address[:], outputID[:])

	return
}

// TransactionID returns the transaction id part of an OutputID.
func (outputID OutputID) TransactionID() (transactionID ID) {
	copy(transactionID[:], outputID[address.Length:])

	return
}

// Bytes marshals the OutputID into a sequence of bytes.
func (outputID OutputID) Bytes() []byte {
	return outputID[:]
}

// String creates a human readable version of the OutputID (for debug purposes).
func (outputID OutputID) String() string {
	return base58.Encode(outputID[:])
}

// OutputIDLength contains the amount of bytes that a marshaled version of the OutputID contains.
const OutputIDLength = address.Length + IDLength
