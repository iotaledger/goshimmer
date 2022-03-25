package evilwallet

import (
	"strconv"
)

func SingleTransactionBatch() EvilBatch {
	return EvilBatch{{ScenarioAlias{inputs: []string{"1"}, outputs: []string{"2"}}}}
}

func DoubleSpendBatch(spentNum int) EvilBatch {
	conflictSlice := make(EvilBatch, 0)
	aliasCounter := 1
	// each double spend uses a different unspent output
	inputStartNum := spentNum * 2

	for i := 1; i <= spentNum; i++ {
		conflictSlice = append(conflictSlice, []ScenarioAlias{
			{inputs: []string{strconv.Itoa(inputStartNum + i)}, outputs: []string{strconv.Itoa(aliasCounter)}},
			{inputs: []string{strconv.Itoa(inputStartNum + i)}, outputs: []string{strconv.Itoa(aliasCounter + 1)}},
		})
		aliasCounter += 2
	}
	return conflictSlice
}
