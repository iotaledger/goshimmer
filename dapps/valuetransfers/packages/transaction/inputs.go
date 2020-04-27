package transaction

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"

	"github.com/iotaledger/hive.go/marshalutil"
)

type Inputs struct {
	*orderedmap.OrderedMap
}

func NewInputs(outputIds ...OutputId) (inputs *Inputs) {
	inputs = &Inputs{orderedmap.New()}
	for _, outputId := range outputIds {
		inputs.Add(outputId)
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

		idBytes, readErr := marshalUtil.ReadBytes(IdLength)
		if readErr != nil {
			err = readErr

			return
		}
		id, idErr, _ := IdFromBytes(idBytes)
		if idErr != nil {
			err = idErr

			return
		}

		addressMap, addressExists := inputs.Get(readAddress)
		if !addressExists {
			addressMap = orderedmap.New()

			inputs.Set(readAddress, addressMap)
		}
		addressMap.(*orderedmap.OrderedMap).Set(id, NewOutputId(readAddress, id))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (inputs *Inputs) Add(input OutputId) *Inputs {
	inputAddress := input.Address()
	transactionId := input.TransactionId()

	addressMap, addressExists := inputs.Get(inputAddress)
	if !addressExists {
		addressMap = orderedmap.New()

		inputs.Set(inputAddress, addressMap)
	}

	addressMap.(*orderedmap.OrderedMap).Set(transactionId, input)

	return inputs
}

func (inputs *Inputs) Bytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(outputId OutputId) bool {
		marshalUtil.WriteBytes(outputId.Bytes())

		inputCounter++

		return true
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

func (inputs *Inputs) ForEach(consumer func(outputId OutputId) bool) bool {
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

func (inputs *Inputs) ForEachTransaction(consumer func(transactionId Id) bool) bool {
	seenTransactions := make(map[Id]bool)

	return inputs.ForEach(func(outputId OutputId) bool {
		if currentTransactionId := outputId.TransactionId(); !seenTransactions[currentTransactionId] {
			seenTransactions[currentTransactionId] = true

			return consumer(currentTransactionId)
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
	inputs.ForEach(func(outputId OutputId) bool {
		empty = false

		result += "    " + outputId.String() + ",\n"

		return true
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
