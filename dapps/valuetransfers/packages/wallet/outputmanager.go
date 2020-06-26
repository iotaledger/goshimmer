package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type OutputManager struct {
	addressManager *AddressManager
	outputs        map[Address]map[transaction.OutputID]Output
}
