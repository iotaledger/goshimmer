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

func NSpendBatch(nSpent int) EvilBatch {
	conflictSlice := make(EvilBatch, 0)
	scenarioAlias := make([]ScenarioAlias, 0)
	inputStartNum := nSpent + 1

	for i := 1; i <= nSpent; i++ {
		scenarioAlias = append(scenarioAlias,
			ScenarioAlias{
				Inputs:  []string{strconv.Itoa(inputStartNum)},
				Outputs: []string{strconv.Itoa(i)}},
		)
	}
	conflictSlice = append(conflictSlice, scenarioAlias)
	return conflictSlice
}

func Scenario1() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"1"}, Outputs: []string{"2", "3"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"2"}, Outputs: []string{"4"}},
			{Inputs: []string{"2"}, Outputs: []string{"5"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"3"}, Outputs: []string{"6"}},
			{Inputs: []string{"3"}, Outputs: []string{"7"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"6", "5"}, Outputs: []string{"8"}},
		},
	}
}

func NoConflictsScenario1() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"1"}, Outputs: []string{"3", "4"}},
			{Inputs: []string{"2"}, Outputs: []string{"5", "6"}},
			{Inputs: []string{"6", "4"}, Outputs: []string{"7"}},
			{Inputs: []string{"3", "5"}, Outputs: []string{"8"}},
			{Inputs: []string{"8", "7"}, Outputs: []string{"11"}},
		},
	}
}
