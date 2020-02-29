package valuetangle

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

type TransferInputs struct {
	*orderedmap.OrderedMap
}

func NewTransferInputs(transferOutputIds ...TransferOutputId) (inputs *TransferInputs) {
	inputs = &TransferInputs{orderedmap.New()}

	for _, transferOutputId := range transferOutputIds {
		inputs.Add(transferOutputId)
	}

	return
}

func TransferInputsFromBytes(bytes []byte) (inputs *TransferInputs, err error, consumedBytes int) {
	inputs = NewTransferInputs()

	marshalUtil := marshalutil.New(bytes)
	inputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	for i := uint32(0); i < inputCount; i++ {
		addressBytes, readErr := marshalUtil.ReadBytes(AddressLength)
		if readErr != nil {
			err = readErr

			return
		}
		address := NewAddress(addressBytes)

		transferIdBytes, readErr := marshalUtil.ReadBytes(TransferIdLength)
		if readErr != nil {
			err = readErr

			return
		}
		transferId := NewTransferId(transferIdBytes)

		addressMap, addressExists := inputs.Get(address)
		if !addressExists {
			addressMap = orderedmap.New()

			inputs.Set(address, addressMap)
		}
		addressMap.(*orderedmap.OrderedMap).Set(transferId, NewTransferOutputId(address, transferId))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (inputs *TransferInputs) Add(input TransferOutputId) *TransferInputs {
	address := input.GetAddress()
	transferId := input.GetTransferId()

	addressMap, addressExists := inputs.Get(address)
	if !addressExists {
		addressMap = orderedmap.New()

		inputs.Set(address, addressMap)
	}

	addressMap.(*orderedmap.OrderedMap).Set(transferId, input)

	return inputs
}

func (inputs *TransferInputs) ToBytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(transferOutputId TransferOutputId) {
		marshalUtil.WriteBytes(transferOutputId.ToBytes())

		inputCounter++
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

func (inputs *TransferInputs) ForEach(consumer func(transferOutputId TransferOutputId)) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		value.(*orderedmap.OrderedMap).ForEach(func(key, value interface{}) bool {
			consumer(value.(TransferOutputId))

			return true
		})

		return true
	})
}

func (inputs *TransferInputs) ForEachAddress(consumer func(address Address)) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		consumer(key.(Address))

		return true
	})
}
