package evilwallet

import "github.com/iotaledger/goshimmer/packages/ledgerstate"

func SplitBalanceEqually(splitNumber int, balance uint64) []uint64 {
	outputBalances := make([]uint64, 0)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	// input is divided equally among outputs
	for i := 0; i < splitNumber-1; i++ {
		outputBalances = append(outputBalances, balance/uint64(splitNumber))
		totalBalance, _ = ledgerstate.SafeAddUint64(totalBalance, outputBalances[i])
	}
	lastBalance, _ := ledgerstate.SafeSubUint64(balance, totalBalance)
	outputBalances = append(outputBalances, lastBalance)

	return outputBalances
}
