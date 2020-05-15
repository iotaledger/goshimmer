package transaction

import (
	"github.com/iotaledger/hive.go/marshalutil"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
)

// Outputs represents a list of Outputs that are part of a transaction.
type Outputs struct {
	*orderedmap.OrderedMap
}

// NewOutputs is the constructor of the Outputs struct and creates the list of Outputs from the given details.
func NewOutputs(outputs map[address.Address][]*balance.Balance) (result *Outputs) {
	result = &Outputs{orderedmap.New()}
	for address, balances := range outputs {
		result.Add(address, balances)
	}

	return
}

// OutputsFromBytes reads the bytes and unmarshals the given information into an *Outputs object. It either creates a
// new object, or uses the optional object provided in the arguments.
func OutputsFromBytes(bytes []byte, optionalTargetObject ...*Outputs) (result *Outputs, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Outputs{orderedmap.New()}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read number of addresses in the outputs
	addressCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	// iterate the corresponding times and collect addresses + their details
	for i := uint32(0); i < addressCount; i++ {
		// read address
		address, addressErr := address.Parse(marshalUtil)
		if addressErr != nil {
			err = addressErr

			return
		}

		// read number of balances in the outputs
		balanceCount, balanceCountErr := marshalUtil.ReadUint32()
		if balanceCountErr != nil {
			err = balanceCountErr

			return
		}

		// iterate the corresponding times and collect balances
		coloredBalances := make([]*balance.Balance, balanceCount)
		for j := uint32(0); j < balanceCount; j++ {
			coloredBalance, coloredBalanceErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return balance.FromBytes(data) })
			if coloredBalanceErr != nil {
				err = coloredBalanceErr

				return
			}

			coloredBalances[j] = coloredBalance.(*balance.Balance)
		}

		// add the gathered information as an output
		result.Add(address, coloredBalances)
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Add adds a new Output to the list of Outputs.
func (outputs *Outputs) Add(address address.Address, balances []*balance.Balance) *Outputs {
	outputs.Set(address, balances)

	return outputs
}

// ForEach iterates through the Outputs and calls them consumer for every found one. The iteration can be aborted by
// returning false in the consumer.
func (outputs *Outputs) ForEach(consumer func(address address.Address, balances []*balance.Balance) bool) bool {
	return outputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		return consumer(key.(address.Address), value.([]*balance.Balance))
	})
}

// Bytes returns a marshaled version of this list of Outputs.
func (outputs *Outputs) Bytes() []byte {
	marshalUtil := marshalutil.New()

	if outputs == nil {
		marshalUtil.WriteUint32(0)

		return marshalUtil.Bytes()
	}

	marshalUtil.WriteUint32(uint32(outputs.Size()))
	outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		marshalUtil.WriteBytes(address.Bytes())
		marshalUtil.WriteUint32(uint32(len(balances)))

		for _, balance := range balances {
			marshalUtil.WriteBytes(balance.Bytes())
		}

		return true
	})

	return marshalUtil.Bytes()
}

// String returns a human readable version of this list of Outputs (for debug purposes).
func (outputs *Outputs) String() string {
	if outputs == nil {
		return "<nil>"
	}

	result := "[\n"
	empty := true
	outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		empty = false

		result += "    " + address.String() + ": [\n"

		balancesEmpty := true
		for _, balance := range balances {
			balancesEmpty = false

			result += "        " + balance.String() + ",\n"
		}

		if balancesEmpty {
			result += "        <empty>\n"
		}

		result += "    ]\n"

		return true
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
