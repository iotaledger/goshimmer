package address

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Address represents an address in a wallet. It extends the normal address type with an index number that was used to
// generate the address from its seed.
type Address struct {
	ledgerstate.Address
	Index uint64
}

// AddressEmpty represents the 0-value of an address.
var AddressEmpty = Address{}
