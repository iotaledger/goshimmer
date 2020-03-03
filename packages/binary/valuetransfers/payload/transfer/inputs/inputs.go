package inputs

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	address2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/id"
	id2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload/transfer/output/id"
)

type Inputs struct {
	*orderedmap.OrderedMap
}

func New(transferOutputIds ...id2.Id) (inputs *Inputs) {
	inputs = &Inputs{orderedmap.New()}
	for _, transferOutputId := range transferOutputIds {
		inputs.Add(transferOutputId)
	}

	return
}

func FromBytes(bytes []byte) (inputs *Inputs, err error, consumedBytes int) {
	inputs = New()

	marshalUtil := marshalutil.New(bytes)
	inputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	for i := uint32(0); i < inputCount; i++ {
		addressBytes, readErr := marshalUtil.ReadBytes(address2.Length)
		if readErr != nil {
			err = readErr

			return
		}
		readAddress := address2.New(addressBytes)

		transferIdBytes, readErr := marshalUtil.ReadBytes(id2.Length)
		if readErr != nil {
			err = readErr

			return
		}
		transferId := id.New(transferIdBytes)

		addressMap, addressExists := inputs.Get(readAddress)
		if !addressExists {
			addressMap = orderedmap.New()

			inputs.Set(readAddress, addressMap)
		}
		addressMap.(*orderedmap.OrderedMap).Set(transferId, id2.New(readAddress, transferId))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (inputs *Inputs) Add(input id2.Id) *Inputs {
	inputAddress := input.GetAddress()
	transferId := input.GetTransferId()

	addressMap, addressExists := inputs.Get(inputAddress)
	if !addressExists {
		addressMap = orderedmap.New()

		inputs.Set(inputAddress, addressMap)
	}

	addressMap.(*orderedmap.OrderedMap).Set(transferId, input)

	return inputs
}

func (inputs *Inputs) ToBytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(transferOutputId id2.Id) {
		marshalUtil.WriteBytes(transferOutputId.ToBytes())

		inputCounter++
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

func (inputs *Inputs) ForEach(consumer func(transferOutputId id2.Id)) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		value.(*orderedmap.OrderedMap).ForEach(func(key, value interface{}) bool {
			consumer(value.(id2.Id))

			return true
		})

		return true
	})
}

func (inputs *Inputs) ForEachAddress(consumer func(currentAddress address2.Address)) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		consumer(key.(address2.Address))

		return true
	})
}

func (inputs *Inputs) String() string {
	if inputs == nil {
		return "<nil>"
	}

	result := "[\n"

	empty := true
	inputs.ForEach(func(transferOutputId id2.Id) {
		empty = false

		result += "    " + transferOutputId.String() + ",\n"
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
