package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type Output struct {
	id             transaction.OutputID
	balances       map[balance.Color]uint64
	inclusionState int
}
