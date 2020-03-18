package inputs

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
	transferid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
	transferoutputid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transferoutput/id"
)

type Inputs struct {
	*orderedmap.OrderedMap
}

func New(transferOutputIds ...transferoutputid.Id) (inputs *Inputs) {
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
		readAddress, addressErr := address.Parse(marshalUtil)
		if addressErr != nil {
			err = addressErr

			return
		}

		transferIdBytes, readErr := marshalUtil.ReadBytes(transferid.Length)
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
		addressMap.(*orderedmap.OrderedMap).Set(transferId, transferoutputid.New(readAddress, transferId))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (inputs *Inputs) Add(input transferoutputid.Id) *Inputs {
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

func (inputs *Inputs) Bytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(transferOutputId transferoutputid.Id) {
		marshalUtil.WriteBytes(transferOutputId.ToBytes())

		inputCounter++
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

func (inputs *Inputs) ForEach(consumer func(transferOutputId transferoutputid.Id)) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		value.(*orderedmap.OrderedMap).ForEach(func(key, value interface{}) bool {
			consumer(value.(transferoutputid.Id))

			return true
		})

		return true
	})
}

func (inputs *Inputs) ForEachAddress(consumer func(currentAddress address.Address) bool) {
	inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return consumer(key.(address.Address))
	})
}

func (inputs *Inputs) String() string {
	if inputs == nil {
		return "<nil>"
	}

	result := "[\n"

	empty := true
	inputs.ForEach(func(transferOutputId transferoutputid.Id) {
		empty = false

		result += "    " + transferOutputId.String() + ",\n"
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
