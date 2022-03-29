package evilwallet

import (
	"strconv"
)

func SingleTransactionBatch() EvilBatch {
	return EvilBatch{{ScenarioAlias{Inputs: []string{"1"}, Outputs: []string{"2"}}}}
}

func DoubleSpendBatch(spentNum int) EvilBatch {
	conflictSlice := make(EvilBatch, 0)
	aliasCounter := 1
	// each double spend uses a different unspent output
	inputStartNum := spentNum * 2

	for i := 1; i <= spentNum; i++ {
		conflictSlice = append(conflictSlice, []ScenarioAlias{
			{Inputs: []string{strconv.Itoa(inputStartNum + i)}, Outputs: []string{strconv.Itoa(aliasCounter)}},
			{Inputs: []string{strconv.Itoa(inputStartNum + i)}, Outputs: []string{strconv.Itoa(aliasCounter + 1)}},
		})
		aliasCounter += 2
	}
	return conflictSlice
}
