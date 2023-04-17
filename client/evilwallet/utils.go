package evilwallet

import (
	"github.com/iotaledger/goshimmer/packages/app/jsonmodels"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
)

// SplitBalanceEqually splits the balance equally between `splitNumber` outputs.
func SplitBalanceEqually(splitNumber int, balance uint64) []uint64 {
	outputBalances := make([]uint64, 0)
	// make sure the output balances are equal input
	var totalBalance uint64 = 0
	// input is divided equally among outputs
	for i := 0; i < splitNumber-1; i++ {
		outputBalances = append(outputBalances, balance/uint64(splitNumber))
		totalBalance, _ = devnetvm.SafeAddUint64(totalBalance, outputBalances[i])
	}
	lastBalance, _ := devnetvm.SafeSubUint64(balance, totalBalance)
	outputBalances = append(outputBalances, lastBalance)

	return outputBalances
}

func getOutputIDsByJSON(outputs []*jsonmodels.Output) (outputIDs []utxo.OutputID) {
	for _, jsonOutput := range outputs {
		output, err := jsonOutput.ToLedgerstateOutput()
		if err != nil {
			continue
		}
		outputIDs = append(outputIDs, output.ID())
	}
	return outputIDs
}

func getOutputByJSON(jsonOutput *jsonmodels.Output) (output devnetvm.Output) {
	output, err := jsonOutput.ToLedgerstateOutput()
	if err != nil {
		return
	}

	return output
}

func getIotaColorAmount(balance *devnetvm.ColoredBalances) uint64 {
	outBalance := uint64(0)
	balance.ForEach(func(color devnetvm.Color, balance uint64) bool {
		if color == devnetvm.ColorIOTA {
			outBalance += balance
		}
		return true
	})
	return outBalance
}

// RateSetterSleep sleeps for the given rate.
func RateSetterSleep(clt Client, useRateSetter bool) error {
	if useRateSetter {
		err := clt.SleepRateSetterEstimate()
		if err != nil {
			return err
		}
	}
	return nil
}
