package outputs

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance"
)

type Outputs struct {
	*orderedmap.OrderedMap
}

func New(outputs map[address.Address][]*coloredbalance.ColoredBalance) (result *Outputs) {
	result = &Outputs{orderedmap.New()}
	for address, balances := range outputs {
		result.Add(address, balances)
	}

	return
}

// FromBytes reads the bytes and unmarshals the given information into an *Outputs object. It either creates a
// new object, or uses the optional object provided in the arguments.
func FromBytes(bytes []byte, optionalTargetObject ...*Outputs) (result *Outputs, err error, consumedBytes int) {
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
		coloredBalances := make([]*coloredbalance.ColoredBalance, balanceCount)
		for j := uint32(0); j < balanceCount; j++ {
			coloredBalance, coloredBalanceErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return coloredbalance.FromBytes(data) })
			if coloredBalanceErr != nil {
				err = coloredBalanceErr

				return
			}

			coloredBalances[j] = coloredBalance.(*coloredbalance.ColoredBalance)
		}

		// add the gathered information as an output
		result.Add(address, coloredBalances)
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (outputs *Outputs) Add(address address.Address, balances []*coloredbalance.ColoredBalance) *Outputs {
	outputs.Set(address, balances)

	return outputs
}

func (outputs *Outputs) ForEach(consumer func(address address.Address, balances []*coloredbalance.ColoredBalance)) {
	outputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		consumer(key.(address.Address), value.([]*coloredbalance.ColoredBalance))

		return true
	})
}

func (outputs *Outputs) Bytes() []byte {
	marshalUtil := marshalutil.New()

	if outputs == nil {
		marshalUtil.WriteUint32(0)

		return marshalUtil.Bytes()
	}

	marshalUtil.WriteUint32(uint32(outputs.Size()))
	outputs.ForEach(func(address address.Address, balances []*coloredbalance.ColoredBalance) {
		marshalUtil.WriteBytes(address.Bytes())
		marshalUtil.WriteUint32(uint32(len(balances)))

		for _, balance := range balances {
			marshalUtil.WriteBytes(balance.Bytes())
		}
	})

	return marshalUtil.Bytes()
}

func (outputs *Outputs) String() string {
	if outputs == nil {
		return "<nil>"
	}

	result := "[\n"
	empty := true
	outputs.ForEach(func(address address.Address, balances []*coloredbalance.ColoredBalance) {
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
	})

	if empty {
		result += "    <empty>\n"
	}

	return result + "]"
}
