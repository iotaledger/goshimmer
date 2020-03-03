package valuetransfers

import (
	"github.com/mr-tron/base58"
)

type TransferOutputId [TransferOutputIdLength]byte

func NewTransferOutputId(address Address, transferId TransferId) (transferOutputId TransferOutputId) {
	copy(transferOutputId[:AddressLength], address.ToBytes())
	copy(transferOutputId[AddressLength:], transferId[:])

	return
}

func TransferOutputIdFromBytes(bytes []byte) (transferOutputId TransferOutputId) {
	copy(transferOutputId[:], bytes)

	return
}

func (transferOutputId TransferOutputId) GetAddress() Address {
	return NewAddress(transferOutputId[:AddressLength])
}

func (transferOutputId TransferOutputId) GetTransferId() TransferId {
	return NewTransferId(transferOutputId[AddressLength:])
}

func (transferOutputId TransferOutputId) ToBytes() []byte {
	return transferOutputId[:]
}

func (transferOutputId TransferOutputId) String() string {
	return "TransferOutputId(" + base58.Encode(transferOutputId[:]) + ")"
}

const TransferOutputIdLength = AddressLength + TransferIdLength
