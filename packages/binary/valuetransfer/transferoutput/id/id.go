package id

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

type Id [Length]byte

func New(outputAddress address.Address, transferId id.Id) (transferOutputId Id) {
	copy(transferOutputId[:address.Length], outputAddress.Bytes())
	copy(transferOutputId[address.Length:], transferId[:])

	return
}

func FromBytes(bytes []byte) (transferOutputId Id) {
	copy(transferOutputId[:], bytes)

	return
}

func (transferOutputId Id) GetAddress() (address address.Address) {
	copy(address[:], transferOutputId[:])

	return
}

func (transferOutputId Id) GetTransferId() id.Id {
	return id.New(transferOutputId[address.Length:])
}

func (transferOutputId Id) ToBytes() []byte {
	return transferOutputId[:]
}

func (transferOutputId Id) String() string {
	return "Id(" + base58.Encode(transferOutputId[:]) + ")"
}

const Length = address.Length + id.Length
