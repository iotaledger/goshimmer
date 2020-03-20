package transferoutput

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

// Id is the data type that represents the identifier for a TransferOutput.
type Id [IdLength]byte

// NewId is the constructor for the Id type.
func NewId(outputAddress address.Address, transferId id.Id) (transferOutputId Id) {
	copy(transferOutputId[:address.Length], outputAddress.Bytes())
	copy(transferOutputId[address.Length:], transferId[:])

	return
}

// IdFromBytes unmarshals an Id from a sequence of bytes.
func IdFromBytes(bytes []byte) (result Id, err error, consumedBytes int) {
	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if idBytes, idErr := marshalUtil.ReadBytes(IdLength); idErr != nil {
		err = idErr

		return
	} else {
		copy(result[:], idBytes)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling of Ids from a byte stream using the marshalUtil package.
func ParseId(marshalUtil *marshalutil.MarshalUtil) (Id, error) {
	if transferMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return IdFromBytes(data) }); err != nil {
		return Id{}, err
	} else {
		return transferMetadata.(Id), nil
	}
}

// Address returns the address part of an Id.
func (transferOutputId Id) Address() (address address.Address) {
	copy(address[:], transferOutputId[:])

	return
}

// TransferId returns the transfer id part of an Id.
func (transferOutputId Id) TransferId() id.Id {
	return id.New(transferOutputId[address.Length:])
}

// Bytes marshals the Id into a sequence of bytes.
func (transferOutputId Id) Bytes() []byte {
	return transferOutputId[:]
}

// String creates a human readable version of the Id (for debug purposes).
func (transferOutputId Id) String() string {
	return base58.Encode(transferOutputId[:])
}

// IdLength contains the amount of bytes that a marshaled version of the Id contains.
const IdLength = address.Length + id.Length
