package address

import (
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/stringify"
)

// Address represents an address in a wallet. It extends the normal address type with an index number that was used to
// generate the address from its seed.
type Address struct {
	AddressBytes [devnetvm.AddressLength]byte
	Index        uint64
}

// Address returns the ledgerstate Address of this wallet Address.
func (a Address) Address() (ledgerStateAddress devnetvm.Address) {
	ledgerStateAddress, _, err := devnetvm.AddressFromBytes(a.AddressBytes[:])
	if err != nil {
		panic(err)
	}

	return
}

// Base58 returns the base58 encoded address.
func (a Address) Base58() string {
	return base58.Encode(a.AddressBytes[:])
}

func (a Address) String() string {
	return stringify.Struct("Address",
		stringify.NewStructField("Address", a.Address()),
		stringify.NewStructField("Index", a.Index),
	)
}

// AddressEmpty represents the 0-value of an address.
var AddressEmpty = Address{}
