package evilwallet

import (
	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// SplitBalanceEqually splits the balance equally between `splitNumber` outputs.
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

func getOutputIDsByJSON(outputs []*jsonmodels.Output) (outputIDs []ledgerstate.OutputID) {
	for _, jsonOutput := range outputs {
		output, err := jsonOutput.ToLedgerstateOutput()
		if err != nil {
			continue
		}
		outputIDs = append(outputIDs, output.ID())
	}
	return outputIDs
}
