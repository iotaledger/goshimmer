package evilwallet

import (
	"fmt"
	"strconv"
)

func SingleTransactionBatch() EvilBatch {
	return EvilBatch{{"A": []Option{WithInputs("1"), WithOutputs([]string{"2"})}}}
}

func DoubleSpendBatch(spentNum int) EvilBatch {
	conflictMap := make(ConflictMap)
	for i := 1; i <= spentNum; i++ {
		conflictName := fmt.Sprintf("C%d", i)
		conflictMap[conflictName] = []Option{WithInputs("0"), WithOutputs([]string{strconv.Itoa(i)})}
	}
	return EvilBatch{conflictMap}
}
