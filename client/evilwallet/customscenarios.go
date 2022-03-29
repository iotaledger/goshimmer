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

func Scenario1() EvilBatch {
	return []ConflictSlice{
		{
			// split funds
			[]Option{WithInputs("1"), WithOutputs([]*OutputOption{{aliasName: "2"}, {aliasName: "3"}})},
		},
		{
			[]Option{WithInputs("2"), WithOutput(&OutputOption{aliasName: "4"})},
			[]Option{WithInputs("2"), WithOutput(&OutputOption{aliasName: "5"})},
		},
		{
			[]Option{WithInputs("3"), WithOutput(&OutputOption{aliasName: "6"})},
			[]Option{WithInputs("3"), WithOutput(&OutputOption{aliasName: "7"})},
		},
		{
			// aggregated
			[]Option{WithInputs("5", "6"), WithOutput(&OutputOption{aliasName: "8", amount: 1000000})},
		},
	}
}
