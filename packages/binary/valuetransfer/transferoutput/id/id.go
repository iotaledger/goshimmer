package id

import (
	"github.com/mr-tron/base58"

	address2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	id2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

type Id [Length]byte

func New(outputAddress address2.Address, transferId id2.Id) (transferOutputId Id) {
	copy(transferOutputId[:address2.Length], outputAddress.ToBytes())
	copy(transferOutputId[address2.Length:], transferId[:])

	return
}

func FromBytes(bytes []byte) (transferOutputId Id) {
	copy(transferOutputId[:], bytes)

	return
}

func (transferOutputId Id) GetAddress() address2.Address {
	return address2.New(transferOutputId[:address2.Length])
}

func (transferOutputId Id) GetTransferId() id2.Id {
	return id2.New(transferOutputId[address2.Length:])
}

func (transferOutputId Id) ToBytes() []byte {
	return transferOutputId[:]
}

func (transferOutputId Id) String() string {
	return "Id(" + base58.Encode(transferOutputId[:]) + ")"
}

const Length = address2.Length + id2.Length
