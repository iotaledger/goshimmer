package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type Output struct {
	address        Address
	transactionID  transaction.ID
	balances       map[balance.Color]uint64
	inclusionState InclusionState
}

type InclusionState struct {
	Liked       bool
	Confirmed   bool
	Rejected    bool
	Conflicting bool
	Spent       bool
}
