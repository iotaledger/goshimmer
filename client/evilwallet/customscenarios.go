package evilwallet

import (
	"strconv"
)

func SingleTransactionBatch() EvilBatch {
	return EvilBatch{{[]Option{WithInputs("1"), WithOutput(&OutputOption{aliasName: "2"})}}}
}

func DoubleSpendBatch(spentNum int) EvilBatch {
	conflictSlice := make(ConflictSlice, 0)
	for i := 1; i <= spentNum; i++ {
		conflictSlice = append(conflictSlice, []Option{WithInputs("0"), WithOutput(&OutputOption{aliasName: strconv.Itoa(i)})})
	}
	return EvilBatch{conflictSlice}
}
