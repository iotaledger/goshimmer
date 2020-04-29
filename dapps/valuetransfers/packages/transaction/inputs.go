package transaction

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"

	"github.com/iotaledger/hive.go/marshalutil"
)

// Inputs represents a list of referenced Outputs that are used as Inputs in a transaction.
type Inputs struct {
	*orderedmap.OrderedMap
}

// NewInputs is the constructor of the Inputs object and creates a new list with the given OutputIds.
func NewInputs(outputIds ...OutputID) (inputs *Inputs) {
	inputs = &Inputs{orderedmap.New()}
	for _, outputID := range outputIds {
		inputs.Add(outputID)
	}

	return
}

// InputsFromBytes unmarshals the Inputs from a sequence of bytes.
func InputsFromBytes(bytes []byte) (inputs *Inputs, consumedBytes int, err error) {
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

		idBytes, readErr := marshalUtil.ReadBytes(IDLength)
		if readErr != nil {
			err = readErr

			return
		}
		id, _, idErr := IDFromBytes(idBytes)
		if idErr != nil {
			err = idErr

			return
		}

		addressMap, addressExists := inputs.Get(readAddress)
		if !addressExists {
			addressMap = orderedmap.New()

			inputs.Set(readAddress, addressMap)
		}
		addressMap.(*orderedmap.OrderedMap).Set(id, NewOutputID(readAddress, id))
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Add allows us to add a new Output to the list of Inputs.
func (inputs *Inputs) Add(input OutputID) *Inputs {
	inputAddress := input.Address()
	transactionID := input.TransactionID()

	addressMap, addressExists := inputs.Get(inputAddress)
	if !addressExists {
		addressMap = orderedmap.New()

		inputs.Set(inputAddress, addressMap)
	}

	addressMap.(*orderedmap.OrderedMap).Set(transactionID, input)

	return inputs
}

// Bytes returns a marshaled version of this list of Inputs.
func (inputs *Inputs) Bytes() (bytes []byte) {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteSeek(4)
	var inputCounter uint32
	inputs.ForEach(func(outputId OutputID) bool {
		marshalUtil.WriteBytes(outputId.Bytes())

		inputCounter++

		return true
	})
	marshalUtil.WriteSeek(0)
	marshalUtil.WriteUint32(inputCounter)

	return marshalUtil.Bytes()
}

// ForEach iterates through the referenced Outputs and calls the consumer function for every Output. The iteration can
// be aborted by returning false in the consumer.
func (inputs *Inputs) ForEach(consumer func(outputId OutputID) bool) bool {
	return inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return value.(*orderedmap.OrderedMap).ForEach(func(key, value interface{}) bool {
			return consumer(value.(OutputID))
		})
	})
}

// ForEachAddress iterates through the input addresses and calls the consumer function for every Address. The iteration
// can be aborted by returning false in the consumer.
func (inputs *Inputs) ForEachAddress(consumer func(currentAddress address.Address) bool) bool {
	return inputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return consumer(key.(address.Address))
	})
}

// ForEachTransaction iterates through the transactions that had their Outputs consumed and calls the consumer function
// for every founds transaction. The iteration can be aborted by returning false in the consumer.
func (inputs *Inputs) ForEachTransaction(consumer func(transactionId ID) bool) bool {
	seenTransactions := make(map[ID]bool)

	return inputs.ForEach(func(outputId OutputID) bool {
		if currentTransactionID := outputId.TransactionID(); !seenTransactions[currentTransactionID] {
			seenTransactions[currentTransactionID] = true

			return consumer(currentTransactionID)
		}

		return true
	})
}

// String returns a human readable version of the list of Inputs (for debug purposes).
func (inputs *Inputs) String() string {
	if inputs == nil {
		return "<nil>"
	}

	result := "[\n"

	empty := true
	inputs.ForEach(func(outputId OutputID) bool {
		empty = false

		result += "    " + outputId.String() + ",\n"

		return true
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
