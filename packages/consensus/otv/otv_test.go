//nolint:dupl
package otv

import (
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
)

func TestOnTangleVoting_LikedInstead(t *testing.T) {
	type ExpectedLikedBranch func(executionBranchAlias string, actualBranchID branchdag.BranchID, actualConflictMembers branchdag.BranchIDs)

	mustMatch := func(s *Scenario, aliasLikedBranches []string, aliasConflictMembers []string) ExpectedLikedBranch {
		return func(_ string, actualBranchID branchdag.BranchID, actualConflictMembers branchdag.BranchIDs) {
			expectedBranches := branchdag.NewBranchIDs()
			expectedConflictMembers := branchdag.NewBranchIDs()
			if len(aliasLikedBranches) > 0 {
				for _, aliasLikedBranch := range aliasLikedBranches {
					expectedBranches.Add(s.BranchID(aliasLikedBranch))
				}
			} else {
				expectedBranches.Add(branchdag.UndefinedBranchID)
			}
			if len(aliasConflictMembers) > 0 {
				for _, aliasConflictMember := range aliasConflictMembers {
					expectedConflictMembers.Add(s.BranchID(aliasConflictMember))
				}
			}
			require.True(t, expectedBranches.Contains(actualBranchID), "expected one of: %s, actual: %s", expectedBranches, actualBranchID)
			require.Equal(t, expectedConflictMembers, actualConflictMembers, "expected: %s, actual: %s", expectedConflictMembers, actualConflictMembers)
		}
	}

	type execution struct {
		branchAlias     string
		wantLikedBranch ExpectedLikedBranch
	}
	type test struct {
		Scenario   Scenario
		WeightFunc consensus.WeightFunc
		executions []execution
	}

	tests := []struct {
		name string
		test test
	}{
		{
			name: "1",
			test: func() test {
				scenario := s1

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "2",
			test: func() test {
				scenario := s2

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "C"}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "3",
			test: func() test {
				scenario := s3

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "4",
			test: func() test {
				scenario := s4

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "4.5",
			test: func() test {
				scenario := s45

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "C"}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "5",
			test: func() test {
				scenario := s5

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "C"}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "6",
			test: func() test {
				scenario := s6

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "C"}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "D"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "7",
			test: func() test {
				scenario := s7

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "D"}, []string{"B", "C", "D"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"D"}, []string{"A", "D"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "C"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "8",
			test: func() test {
				scenario := s8

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B", "C", "D"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "E"}, []string{"A", "E"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A", "D"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"A"}, []string{"A", "C"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "9",
			test: func() test {
				scenario := s9

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B", "D"}, []string{"B", "C", "D"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "E"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"D"}, []string{"A", "D"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "C"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "10",
			test: func() test {
				scenario := s10

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"C"}, []string{"B", "C"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"C"}, []string{"A", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "B"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "12",
			test: func() test {
				scenario := s12

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"E"}, []string{"E"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"D"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "13",
			test: func() test {
				scenario := s13

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "C"}, []string{"A", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"E"}, []string{"E"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"D"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "14",
			test: func() test {
				scenario := s14

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "C"}, []string{"A", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"E"}, []string{"E"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"D"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{"G"}, []string{"G"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"I"}),
					},
					{
						branchAlias:     "I",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"H"}),
					},
					{
						branchAlias:     "J",
						wantLikedBranch: mustMatch(&scenario, []string{"K"}, []string{"K"}),
					},
					{
						branchAlias:     "K",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"J"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "15",
			test: func() test {
				scenario := s15

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "D",
						wantLikedBranch: mustMatch(&scenario, []string{"E"}, []string{"E"}),
					},
					{
						branchAlias:     "E",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"D"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"G"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"I"}),
					},
					{
						branchAlias:     "I",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"H"}),
					},
					{
						branchAlias:     "J",
						wantLikedBranch: mustMatch(&scenario, []string{"K"}, []string{"K"}),
					},
					{
						branchAlias:     "K",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"J"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "16",
			test: func() test {
				scenario := s16

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "H", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B", "H"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"G", "H"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F", "H"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B", "C", "F", "G"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "17",
			test: func() test {
				scenario := s17

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "C"}, []string{"A", "H", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"H", "B"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{"G"}, []string{"G", "H"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F", "H"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "C", "G"}, []string{"B", "C", "F", "G"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "18",
			test: func() test {
				scenario := s18

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "H"}, []string{"A", "H", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"B", "H"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"G", "H"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"F", "H"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F", "G", "B", "C"}),
					},
					{
						branchAlias:     "I",
						wantLikedBranch: mustMatch(&scenario, []string{"J"}, []string{"J", "O"}),
					},
					{
						branchAlias:     "J",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"I", "O"}),
					},
					{
						branchAlias:     "K",
						wantLikedBranch: mustMatch(&scenario, []string{"L"}, []string{"L"}),
					},
					{
						branchAlias:     "L",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"K"}),
					},
					{
						branchAlias:     "M",
						wantLikedBranch: mustMatch(&scenario, []string{"N"}, []string{"N", "O"}),
					},
					{
						branchAlias:     "N",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"M", "O"}),
					},
					{
						branchAlias:     "O",
						wantLikedBranch: mustMatch(&scenario, []string{"J", "N"}, []string{"M", "N", "J", "I"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "19",
			test: func() test {
				scenario := s19

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{"A", "H"}, []string{"A", "H", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"B", "H"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"G", "H"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{"H"}, []string{"F", "H"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F", "G", "B", "C"}),
					},
					{
						branchAlias:     "I",
						wantLikedBranch: mustMatch(&scenario, []string{"O"}, []string{"J", "O"}),
					},
					{
						branchAlias:     "J",
						wantLikedBranch: mustMatch(&scenario, []string{"O"}, []string{"I", "O"}),
					},
					{
						branchAlias:     "K",
						wantLikedBranch: mustMatch(&scenario, []string{"L"}, []string{"L"}),
					},
					{
						branchAlias:     "L",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"K"}),
					},
					{
						branchAlias:     "M",
						wantLikedBranch: mustMatch(&scenario, []string{"O"}, []string{"N", "O"}),
					},
					{
						branchAlias:     "N",
						wantLikedBranch: mustMatch(&scenario, []string{"O"}, []string{"M", "O"}),
					},
					{
						branchAlias:     "O",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"M", "N", "J", "I"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
		{
			name: "20",
			test: func() test {
				scenario := s20

				executions := []execution{
					{
						branchAlias:     "A",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B"}),
					},
					{
						branchAlias:     "B",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"A", "H", "C"}),
					},
					{
						branchAlias:     "C",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B", "H"}),
					},
					{
						branchAlias:     "F",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"G", "H"}),
					},
					{
						branchAlias:     "G",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"F", "H"}),
					},
					{
						branchAlias:     "H",
						wantLikedBranch: mustMatch(&scenario, []string{"B"}, []string{"B", "C", "F", "G"}),
					},
					{
						branchAlias:     "I",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"J"}),
					},
					{
						branchAlias:     "J",
						wantLikedBranch: mustMatch(&scenario, []string{}, []string{"I"}),
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					executions: executions,
				}
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := ledger.New(ledger.WithCacheTimeProvider(database.NewCacheTimeProvider(0)))
			defer ls.Shutdown()

			tt.test.Scenario.CreateBranches(t, ls.BranchDAG)
			o := NewOnTangleVoting(ls.BranchDAG, tt.test.WeightFunc)

			for _, e := range tt.test.executions {
				liked, conflictMembers := o.LikedConflictMember(tt.test.Scenario.BranchID(e.branchAlias))
				fmt.Println("branchAlias", e.branchAlias)
				e.wantLikedBranch(e.branchAlias, liked, conflictMembers)
			}
		})
	}
}

// region test helpers /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchMeta describes a branch in a branchDAG with its conflicts and approval weight.
type BranchMeta struct {
	Order          int
	BranchID       branchdag.BranchID
	ParentBranches branchdag.BranchIDs
	Conflicting    branchdag.ConflictIDs
	ApprovalWeight float64
}

// Scenario is a testing utility representing a branchDAG with additional information such as approval weight for each
// individual branch.
type Scenario map[string]*BranchMeta

// IDsToNames returns a mapping of BranchIDs to their alias.
func (s *Scenario) IDsToNames() map[branchdag.BranchID]string {
	mapping := map[branchdag.BranchID]string{}
	for name, m := range *s {
		mapping[m.BranchID] = name
	}
	return mapping
}

// BranchID returns the BranchID of the given branch alias.
func (s *Scenario) BranchID(alias string) branchdag.BranchID {
	return (*s)[alias].BranchID
}

// BranchIDs returns either all BranchIDs in the scenario or only the ones with the given aliases.
func (s *Scenario) BranchIDs(aliases ...string) branchdag.BranchIDs {
	branchIDs := branchdag.NewBranchIDs()
	for name, meta := range *s {
		if len(aliases) > 0 {
			var has bool
			for _, alias := range aliases {
				if alias == name {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		branchIDs.Add(meta.BranchID)
	}
	return branchIDs
}

// CreateBranches orders and creates the branches for the scenario.
func (s *Scenario) CreateBranches(t *testing.T, branchDAG *branchdag.BranchDAG) {
	type order struct {
		order int
		name  string
	}

	var ordered []order
	for name, m := range *s {
		ordered = append(ordered, order{order: m.Order, name: name})
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].order < ordered[j].order
	})

	for _, o := range ordered {
		m := (*s)[o.name]
		createTestBranch(t, branchDAG, o.name, m)
	}
}

// creates a branch and registers a BranchIDAlias with the name specified in branchMeta.
func createTestBranch(t *testing.T, branchDAG *branchdag.BranchDAG, alias string, branchMeta *BranchMeta) bool {
	var cachedBranch *objectstorage.CachedObject[*branchdag.Branch]
	var newBranchCreated bool

	if branchMeta.BranchID == branchdag.UndefinedBranchID {
		panic("a branch must have its ID defined in its BranchMeta")
	}
	newBranchCreated = branchDAG.CreateBranch(branchMeta.BranchID, branchMeta.ParentBranches, branchMeta.Conflicting)
	require.True(t, newBranchCreated)
	branchDAG.Storage.CachedBranch(branchMeta.BranchID).Consume(func(branch *branchdag.Branch) {
		branch, _ = cachedBranch.Unwrap()
		branchMeta.BranchID = branch.ID()
	})
	branchMeta.BranchID.RegisterAlias(alias)
	return newBranchCreated
}

// WeightFuncFromScenario creates a WeightFunc from the given scenario so that the approval weight can be mocked
// according to the branch weight's specified in the scenario.
func WeightFuncFromScenario(t *testing.T, scenario Scenario) consensus.WeightFunc {
	branchIDsToName := scenario.IDsToNames()
	return func(branchID branchdag.BranchID) (weight float64) {
		name, nameOk := branchIDsToName[branchID]
		require.True(t, nameOk)
		meta, metaOk := scenario[name]
		require.True(t, metaOk)
		return meta.ApprovalWeight
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Scenario definition according to images/otv-testcases.png ////////////////////////////////////////////////////

var (
	s1 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.6,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
	}

	s2 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.6,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.8,
		},
	}

	s3 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{types.Identifier{2}}),
			ApprovalWeight: 0.5,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
	}

	s4 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
	}

	s45 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{200}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
	}

	s5 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.1,
		},
	}

	s6 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{7}}),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{7}}),
			ApprovalWeight: 0.1,
		},
	}

	s7 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.15,
		},
	}

	s8 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}, branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{6}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}),
			ApprovalWeight: 0.5,
		},
	}

	s9 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}, branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{6}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}),
			ApprovalWeight: 0.1,
		},
	}

	s10 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}, branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{0}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
	}

	s12 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.25,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{6}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.35,
		},
	}

	s13 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{6}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.35,
		},
	}

	s14 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{2}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{3}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{1}}, branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{4}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{5}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{6}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{3}}),
			ApprovalWeight: 0.35,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{7}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.BranchID{Identifier: types.Identifier{2}}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{4}}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{8}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.BranchID{Identifier: types.Identifier{2}}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{4}}),
			ApprovalWeight: 0.17,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{9}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.BranchID{Identifier: types.Identifier{6}}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{10}}),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       branchdag.BranchID{Identifier: types.Identifier{10}},
			ParentBranches: branchdag.NewBranchIDs(branchdag.BranchID{Identifier: types.Identifier{6}}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{10}}),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       branchdag.BranchID{11},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{15}),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       branchdag.BranchID{12},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{15}),
			ApprovalWeight: 0.06,
		},
	}

	s15 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       branchdag.BranchID{5},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{3}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       branchdag.BranchID{6},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{3}),
			ApprovalWeight: 0.35,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.17,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(BranchID{6}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{10}),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       branchdag.BranchID{10},
			ParentBranches: branchdag.NewBranchIDs(BranchID{6}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{10}),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       branchdag.BranchID{11},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{15}),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       branchdag.BranchID{12},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{15}),
			ApprovalWeight: 0.06,
		},
	}

	s16 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID, branchdag.BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}, branchdag.ConflictID{4}),
			ApprovalWeight: 0.15,
		},
	}

	s17 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID, branchdag.BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}, branchdag.ConflictID{4}),
			ApprovalWeight: 0.15,
		},
	}

	s18 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID, branchdag.BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}, branchdag.ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       branchdag.BranchID{10},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{17}),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       branchdag.BranchID{11},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{17}),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       branchdag.BranchID{12},
			ParentBranches: branchdag.NewBranchIDs(BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{19}),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       branchdag.BranchID{13},
			ParentBranches: branchdag.NewBranchIDs(BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{19}),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       branchdag.BranchID{14},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       branchdag.BranchID{15},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       branchdag.BranchID{16},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}, branchdag.BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}, branchdag.ConflictID{19}),
			ApprovalWeight: 0.05,
		},
	}

	s19 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID, branchdag.BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}, branchdag.ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       branchdag.BranchID{10},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{17}),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       branchdag.BranchID{11},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{17}),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       branchdag.BranchID{12},
			ParentBranches: branchdag.NewBranchIDs(BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{19}),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       branchdag.BranchID{13},
			ParentBranches: branchdag.NewBranchIDs(BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{19}),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       branchdag.BranchID{14},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       branchdag.BranchID{15},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       branchdag.BranchID{16},
			ParentBranches: branchdag.NewBranchIDs(BranchID{9}, branchdag.BranchID{11}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{14}, branchdag.ConflictID{19}),
			ApprovalWeight: 0.09,
		},
	}

	s20 = Scenario{
		"A": {
			BranchID:       branchdag.BranchID{2},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       branchdag.BranchID{3},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{1}, branchdag.ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       branchdag.BranchID{4},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       branchdag.BranchID{7},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       branchdag.BranchID{8},
			ParentBranches: branchdag.NewBranchIDs(BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       branchdag.BranchID{9},
			ParentBranches: branchdag.NewBranchIDs(branchdag.MasterBranchID, branchdag.BranchID{2}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{Identifier: types.Identifier{2}}, branchdag.ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"I": {
			Order:          2,
			BranchID:       branchdag.BranchID{10},
			ParentBranches: branchdag.NewBranchIDs(BranchID{7}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{12}),
			ApprovalWeight: 0.005,
		},
		"J": {
			Order:          2,
			BranchID:       branchdag.BranchID{11},
			ParentBranches: branchdag.NewBranchIDs(BranchID{7}),
			Conflicting:    branchdag.NewConflictIDs(branchdag.ConflictID{12}),
			ApprovalWeight: 0.015,
		},
	}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
