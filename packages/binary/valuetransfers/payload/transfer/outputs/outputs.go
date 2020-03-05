package outputs

import (
	"github.com/iotaledger/goshimmer/packages/binary/datastructure/orderedmap"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	address2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/address"
	coloredbalance2 "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/coloredbalance"
)

type Outputs struct {
	*orderedmap.OrderedMap
}

func New(outputs map[address2.Address][]*coloredbalance2.ColoredBalance) (result *Outputs) {
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
	addressCount, addressCountErr := marshalUtil.ReadUint32()
	if addressCountErr != nil {
		err = addressCountErr

		return
	}

	// iterate the corresponding times and collect addresses + their details
	for i := uint32(0); i < addressCount; i++ {
		// read address
		addressBytes, addressErr := marshalUtil.ReadBytes(address2.Length)
		if addressErr != nil {
			err = addressErr

			return
		}
		address := address2.New(addressBytes)

		// read number of balances in the outputs
		balanceCount, balanceCountErr := marshalUtil.ReadUint32()
		if balanceCountErr != nil {
			err = balanceCountErr

			return
		}

		// iterate the corresponding times and collect balances
		coloredBalances := make([]*coloredbalance2.ColoredBalance, balanceCount)
		for j := uint32(0); j < balanceCount; j++ {
			coloredBalance, coloredBalanceErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return coloredbalance2.FromBytes(data) })
			if coloredBalanceErr != nil {
				err = coloredBalanceErr

				return
			}

			coloredBalances[j] = coloredBalance.(*coloredbalance2.ColoredBalance)
		}

		// add the gathered information as an output
		result.Add(address, coloredBalances)
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (outputs *Outputs) Add(address address2.Address, balances []*coloredbalance2.ColoredBalance) *Outputs {
	outputs.Set(address, balances)

	return outputs
}

func (outputs *Outputs) ForEach(consumer func(address address2.Address, balances []*coloredbalance2.ColoredBalance)) {
	outputs.OrderedMap.ForEach(func(key, value interface{}) bool {
		consumer(key.(address2.Address), value.([]*coloredbalance2.ColoredBalance))

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
	outputs.ForEach(func(address address2.Address, balances []*coloredbalance2.ColoredBalance) {
		marshalUtil.WriteBytes(address.ToBytes())
		marshalUtil.WriteUint32(uint32(len(balances)))

		for _, balance := range balances {
			marshalUtil.WriteBytes(balance.ToBytes())
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
	outputs.ForEach(func(address address2.Address, balances []*coloredbalance2.ColoredBalance) {
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
