//nolint:dupl
package otv

import (
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/types"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"

	"github.com/iotaledger/goshimmer/packages/database"
)

func TestOnTangleVoting_LikedInstead(t *testing.T) {
	type ExpectedLikedBranch func(executionBranchAlias string, actualBranchID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID])

	mustMatch := func(s *Scenario, aliasLikedBranches []string, aliasConflictMembers []string) ExpectedLikedBranch {
		return func(_ string, actualBranchID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID]) {
			expectedBranches := set.NewAdvancedSet[utxo.TransactionID]()
			expectedConflictMembers := set.NewAdvancedSet[utxo.TransactionID]()
			if len(aliasLikedBranches) > 0 {
				for _, aliasLikedBranch := range aliasLikedBranches {
					expectedBranches.Add(s.BranchID(aliasLikedBranch))
				}
			} else {
				expectedBranches.Add(utxo.EmptyTransactionID)
			}
			if len(aliasConflictMembers) > 0 {
				for _, aliasConflictMember := range aliasConflictMembers {
					expectedConflictMembers.Add(s.BranchID(aliasConflictMember))
				}
			}
			require.True(t, expectedBranches.Has(actualBranchID), "expected one of: %s, actual: %s", expectedBranches, actualBranchID)
			require.True(t, expectedConflictMembers.Equal(actualConflictMembers), "expected: %s, actual: %s", expectedConflictMembers, actualConflictMembers)
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

			tt.test.Scenario.CreateBranches(t, ls.ConflictDAG)
			o := NewOnTangleVoting(ls.ConflictDAG, tt.test.WeightFunc)

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
	BranchID       utxo.TransactionID
	ParentBranches *set.AdvancedSet[utxo.TransactionID]
	Conflicting    *set.AdvancedSet[utxo.OutputID]
	ApprovalWeight float64
}

// Scenario is a testing utility representing a branchDAG with additional information such as approval weight for each
// individual branch.
type Scenario map[string]*BranchMeta

// IDsToNames returns a mapping of BranchIDs to their alias.
func (s *Scenario) IDsToNames() map[utxo.TransactionID]string {
	mapping := map[utxo.TransactionID]string{}
	for name, m := range *s {
		mapping[m.BranchID] = name
	}
	return mapping
}

// BranchID returns the BranchID of the given branch alias.
func (s *Scenario) BranchID(alias string) utxo.TransactionID {
	return (*s)[alias].BranchID
}

// BranchIDs returns either all BranchIDs in the scenario or only the ones with the given aliases.
func (s *Scenario) BranchIDs(aliases ...string) *set.AdvancedSet[utxo.TransactionID] {
	branchIDs := set.NewAdvancedSet[utxo.TransactionID]()
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
func (s *Scenario) CreateBranches(t *testing.T, branchDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) {
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
func createTestBranch(t *testing.T, branchDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], alias string, branchMeta *BranchMeta) bool {
	var newBranchCreated bool

	if branchMeta.BranchID == utxo.EmptyTransactionID {
		panic("a branch must have its ID defined in its BranchMeta")
	}
	newBranchCreated = branchDAG.CreateConflict(branchMeta.BranchID, branchMeta.ParentBranches, branchMeta.Conflicting)
	require.True(t, newBranchCreated)
	branchDAG.Storage.CachedBranch(branchMeta.BranchID).Consume(func(branch *conflictdag.Branch[utxo.TransactionID, utxo.OutputID]) {
		branchMeta.BranchID = branch.ID()
	})
	branchMeta.BranchID.RegisterAlias(alias)
	return newBranchCreated
}

// WeightFuncFromScenario creates a WeightFunc from the given scenario so that the approval weight can be mocked
// according to the branch weight's specified in the scenario.
func WeightFuncFromScenario(t *testing.T, scenario Scenario) consensus.WeightFunc {
	branchIDsToName := scenario.IDsToNames()
	return func(branchID utxo.TransactionID) (weight float64) {
		name, nameOk := branchIDsToName[branchID]
		require.True(t, nameOk)
		meta, metaOk := scenario[name]
		require.True(t, metaOk)
		return meta.ApprovalWeight
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Scenario definition according to images/otv-testcases.png ////////////////////////////////////////////////////

func newConflictID() (conflictID utxo.OutputID) {
	if err := conflictID.FromRandomness(); err != nil {
		panic(err)
	}
	return conflictID
}

var (
	conflictID0  = newConflictID()
	conflictID1  = newConflictID()
	conflictID2  = newConflictID()
	conflictID3  = newConflictID()
	conflictID4  = newConflictID()
	conflictID5  = newConflictID()
	conflictID6  = newConflictID()
	conflictID7  = newConflictID()
	conflictID8  = newConflictID()
	conflictID9  = newConflictID()
	conflictID11 = newConflictID()
	conflictID12 = newConflictID()

	s1 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.6,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
	}

	s2 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.6,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.8,
		},
	}

	s3 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.5,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
	}

	s4 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.3,
		},
	}

	s45 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{200}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.3,
		},
	}

	s5 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.1,
		},
	}

	s6 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID5),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID5),
			ApprovalWeight: 0.1,
		},
	}

	s7 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.15,
		},
	}

	s8 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0),
			ApprovalWeight: 0.5,
		},
	}

	s9 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0),
			ApprovalWeight: 0.1,
		},
	}

	s10 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID0, conflictID2),
			ApprovalWeight: 0.3,
		},
	}

	s12 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.25,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.35,
		},
	}

	s13 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.35,
		},
	}

	s14 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.35,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.17,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:    set.NewAdvancedSet(conflictID6),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:    set.NewAdvancedSet(conflictID6),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID9),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID9),
			ApprovalWeight: 0.06,
		},
	}

	s15 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID3),
			ApprovalWeight: 0.35,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.17,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:    set.NewAdvancedSet(conflictID6),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:    set.NewAdvancedSet(conflictID6),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID9),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID9),
			ApprovalWeight: 0.06,
		},
	}

	s16 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight: 0.15,
		},
	}

	s17 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight: 0.15,
		},
	}

	s18 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID11),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID11),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID12),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID12),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID8),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID8),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight: 0.05,
		},
	}

	s19 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID11),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID11),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID12),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID12),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID8),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:    set.NewAdvancedSet(conflictID8),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:    set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight: 0.09,
		},
	}

	s20 = Scenario{
		"A": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:    set.NewAdvancedSet(conflictID2),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID4),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentBranches: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:    set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight: 0.15,
		},
		"I": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:    set.NewAdvancedSet(conflictID7),
			ApprovalWeight: 0.005,
		},
		"J": {
			Order:          2,
			BranchID:       utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentBranches: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:    set.NewAdvancedSet(conflictID7),
			ApprovalWeight: 0.015,
		},
	}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
