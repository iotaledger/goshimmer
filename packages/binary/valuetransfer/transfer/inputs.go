package transfer

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
)

type Inputs struct {
	*orderedmap.OrderedMap
}

func NewInputs(transferOutputIds ...OutputId) (inputs *Inputs) {
	inputs = &Inputs{orderedmap.New()}
	for _, transferOutputId := range transferOutputIds {
		inputs.Add(transferOutputId)
	}

	return
}

func InputsFromBytes(bytes []byte) (inputs *Inputs, err error, consumedBytes int) {
	inputs = NewInputs()

	marshalUtil := marshalutil.New(bytes)
	inputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	for i := uint32(0); i < inputCount; i++ {
		readAddress, addressErr := address.Parse(marshalUtil)
		if addressErr != nil {
			err = addressErr

			return
		}

		transferIdBytes, readErr := marshalUtil.ReadBytes(IdLength)
		if readErr != nil {
			err = readErr

			return
		}
		transferId, transferIdErr, _ := IdFromBytes(transferIdBytes)
		if transferIdErr != nil {
			err = transferIdErr

			return
		}

		addressMap, addressExists := inputs.Get(readAddress)
		if !addressExists {
			addressMap = orderedmap.New()

			inputs.Set(readAddress, addressMap)
		}
		addressMap.(*orderedmap.OrderedMap).Set(transferId, NewOutputId(readAddress, transferId))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (inputs *Inputs) Add(input OutputId) *Inputs {
	inputAddress := input.Address()
	transferId := input.TransferId()

	addressMap, addressExists := inputs.Get(inputAddress)
	if !addressExists {
		addressMap = orderedmap.New()

		inputs.Set(inputAddress, addressMap)
	}

	addressMap.(*orderedmap.OrderedMap).Set(transferId, input)

	return inputs
}

func (inputs *Inputs) Bytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(transferOutputId OutputId) bool {
		marshalUtil.WriteBytes(transferOutputId.Bytes())

		inputCounter++

		return true
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

func (inputs *Inputs) ForEach(consumer func(transferOutputId OutputId) bool) bool {
	return inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return value.(*orderedmap.OrderedMap).ForEach(func(key, value interface{}) bool {
			return consumer(value.(OutputId))
		})
	})
}

func (inputs *Inputs) ForEachAddress(consumer func(currentAddress address.Address) bool) bool {
	return inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return consumer(key.(address.Address))
	})
}

func (inputs *Inputs) ForEachTransfer(consumer func(currentTransfer Id) bool) bool {
	seenTransfers := make(map[Id]bool)

	return inputs.ForEach(func(transferOutputId OutputId) bool {
		if currentTransferId := transferOutputId.TransferId(); !seenTransfers[currentTransferId] {
			seenTransfers[currentTransferId] = true

			return consumer(currentTransferId)
		}

		return true
	})
}

func (inputs *Inputs) String() string {
	if inputs == nil {
		return "<nil>"
	}

	result := "[\n"

	empty := true
	inputs.ForEach(func(transferOutputId OutputId) bool {
		empty = false

		result += "    " + transferOutputId.String() + ",\n"

		return true
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
