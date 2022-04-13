package evilwallet

import (
	"strconv"
)

var scenariosMap map[string]EvilBatch

func init() {
	scenariosMap = make(map[string]EvilBatch)
	scenariosMap["tx"] = SingleTransactionBatch()
	scenariosMap["ds"] = NSpendBatch(2)
	scenariosMap["conflict-circle"] = ConflictSetCircle(4)
	scenariosMap["guava"] = Scenario1()
	scenariosMap["orange"] = Scenario2()
	scenariosMap["mango"] = Scenario3()
	scenariosMap["pear"] = Scenario4()
	scenariosMap["lemon"] = Scenario5()
	scenariosMap["banana"] = Scenario6()
	scenariosMap["kiwi"] = Scenario7()
	scenariosMap["peace"] = NoConflictsScenario1()
}

// GetScenario returns an evil batch based i=on its name.
func GetScenario(scenarioName string) (batch EvilBatch, ok bool) {
	batch, ok = scenariosMap[scenarioName]
	return
}

// SingleTransactionBatch returns an EvilBatch that is a single transaction.
func SingleTransactionBatch() EvilBatch {
	return EvilBatch{{ScenarioAlias{Inputs: []string{"1"}, Outputs: []string{"2"}}}}
}

// DoubleSpendBatch returns an EvilBatch that is a spentNum spend.
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

// ConflictSetCircle creates a circular conflict set for a given size, e.g. for size=3, conflict set: A-B-C-A.
func ConflictSetCircle(size int) EvilBatch {
	scenarioAlias := make([]ScenarioAlias, 0)
	inputStartNum := size

	for i := 0; i < inputStartNum; i++ {
		in := i
		in2 := (in + 1) % inputStartNum
		scenarioAlias = append(scenarioAlias,
			ScenarioAlias{
				Inputs:  []string{strconv.Itoa(in), strconv.Itoa(in2)},
				Outputs: []string{strconv.Itoa(inputStartNum + i)},
			})
	}
	return EvilBatch{scenarioAlias}
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

// Scenario1 describes two double spends and aggregates them.
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

// Scenario2 is a reflection of UTXO unit test scenario example B - packages/ledgerstate/utxo_dag_test_exampleB.png
func Scenario2() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"A"}, Outputs: []string{"C"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"B"}, Outputs: []string{"D"}},
			{Inputs: []string{"B"}, Outputs: []string{"G"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"C", "D"}, Outputs: []string{"E"}},
			{Inputs: []string{"D"}, Outputs: []string{"F"}},
		},
	}
}

// Scenario3 is a reflection of UTXO unit test scenario example C - packages/ledgerstate/utxo_dag_test_exampleC.png
func Scenario3() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"A"}, Outputs: []string{"C"}},
			{Inputs: []string{"A"}, Outputs: []string{"D"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"B"}, Outputs: []string{"E"}},
			{Inputs: []string{"B"}, Outputs: []string{"F"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"D", "E"}, Outputs: []string{"G"}},
		},
	}
}

// Scenario4 is a reflection of ledgerstate unit test for branch confirmation - packages/ledgerstate/ledgerstate_test_SetBranchConfirmed.png
func Scenario4() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"1"}, Outputs: []string{"A"}},
			{Inputs: []string{"1"}, Outputs: []string{"B"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"2"}, Outputs: []string{"C"}},
			{Inputs: []string{"2"}, Outputs: []string{"D"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"3"}, Outputs: []string{"E"}},
			{Inputs: []string{"3"}, Outputs: []string{"F"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"D", "E"}, Outputs: []string{"G"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"G"}, Outputs: []string{"H"}},
			{Inputs: []string{"G"}, Outputs: []string{"I"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"A", "I"}, Outputs: []string{"J"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"J"}, Outputs: []string{"K"}},
		},
	}
}

// Scenario5 uses ConflictSetCircle with size 4 and aggregate its outputs.
func Scenario5() EvilBatch {
	circularConflict := ConflictSetCircle(4)
	circularConflict = append(circularConflict, []ScenarioAlias{{
		Inputs:  []string{"4", "6"},
		Outputs: []string{"8"},
	}})
	circularConflict = append(circularConflict, []ScenarioAlias{{
		Inputs:  []string{"5", "7"},
		Outputs: []string{"9"},
	}})
	return circularConflict
}

// Scenario6 returns 5 levels deep scenario.
func Scenario6() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"1"}, Outputs: []string{"A", "B"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"A"}, Outputs: []string{"C", "D"}},
			{Inputs: []string{"B"}, Outputs: []string{"E"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"C"}, Outputs: []string{"F", "G"}},
			{Inputs: []string{"C", "D"}, Outputs: []string{"H"}},
			{Inputs: []string{"D"}, Outputs: []string{"I"}},
			{Inputs: []string{"F", "D"}, Outputs: []string{"J"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"G"}, Outputs: []string{"K"}},
			{Inputs: []string{"I"}, Outputs: []string{"L"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"K", "I"}, Outputs: []string{"M"}},
			{Inputs: []string{"L"}, Outputs: []string{"N"}},
		},
	}
}

// Scenario7 three level deep scenario, with two separate conflict sets aggregated.
func Scenario7() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"A"}, Outputs: []string{"E"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"A", "B"}, Outputs: []string{"F"}},
			{Inputs: []string{"B"}, Outputs: []string{"G"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"C"}, Outputs: []string{"H"}},
			{Inputs: []string{"D"}, Outputs: []string{"I"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"E", "G"}, Outputs: []string{"J"}},
			{Inputs: []string{"H"}, Outputs: []string{"K"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"J", "K"}, Outputs: []string{"L"}},
			{Inputs: []string{"I", "K"}, Outputs: []string{"M"}},
		},
	}
}

// NoConflictsScenario1 returns batch with no conflicts that is 3 levels deep.
func NoConflictsScenario1() EvilBatch {
	return EvilBatch{
		[]ScenarioAlias{
			{Inputs: []string{"1"}, Outputs: []string{"3", "4"}},
			{Inputs: []string{"2"}, Outputs: []string{"5", "6"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"6", "4"}, Outputs: []string{"7"}},
			{Inputs: []string{"3", "5"}, Outputs: []string{"8"}},
		},
		[]ScenarioAlias{
			{Inputs: []string{"8", "7"}, Outputs: []string{"9"}},
		},
	}
}
