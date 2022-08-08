//nolint:dupl
package otv

import (
	"sort"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/consensus"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

func TestOnTangleVoting_LikedInstead(t *testing.T) {
	type ExpectedLikedConflict func(executionConflictAlias string, actualConflictID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID])

	mustMatch := func(s *Scenario, aliasLikedConflicts []string, aliasConflictMembers []string) ExpectedLikedConflict {
		return func(_ string, actualConflictID utxo.TransactionID, actualConflictMembers *set.AdvancedSet[utxo.TransactionID]) {
			expectedConflicts := set.NewAdvancedSet[utxo.TransactionID]()
			expectedConflictMembers := set.NewAdvancedSet[utxo.TransactionID]()
			if len(aliasLikedConflicts) > 0 {
				for _, aliasLikedConflict := range aliasLikedConflicts {
					expectedConflicts.Add(s.ConflictID(aliasLikedConflict))
				}
			} else {
				expectedConflicts.Add(utxo.EmptyTransactionID)
			}
			if len(aliasConflictMembers) > 0 {
				for _, aliasConflictMember := range aliasConflictMembers {
					expectedConflictMembers.Add(s.ConflictID(aliasConflictMember))
				}
			}
			require.True(t, expectedConflicts.Has(actualConflictID), "expected one of: %s, actual: %s", expectedConflicts, actualConflictID)
			require.True(t, expectedConflictMembers.Equal(actualConflictMembers), "expected: %s, actual: %s", expectedConflictMembers, actualConflictMembers)
		}
	}

	type execution struct {
		conflictAlias     string
		wantLikedConflict ExpectedLikedConflict
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B", "C"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"C"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"C"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B", "C"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "D"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"D"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C", "D"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"D"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"D"}, []string{"A", "C"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B", "C", "D"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"B", "A"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"C", "D"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"C", "D"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"B"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C", "D"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "E"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"D"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"D"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"E"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A", "B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A", "B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"A", "B"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"C"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"B"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"B"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{"G"}, []string{"F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{"G"}, []string{"F"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"I"}),
					},
					{
						conflictAlias:     "I",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"I"}),
					},
					{
						conflictAlias:     "J",
						wantLikedConflict: mustMatch(&scenario, []string{"K"}, []string{"J"}),
					},
					{
						conflictAlias:     "K",
						wantLikedConflict: mustMatch(&scenario, []string{"K"}, []string{"J"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"C"}),
					},
					{
						conflictAlias:     "D",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "E",
						wantLikedConflict: mustMatch(&scenario, []string{"E"}, []string{"D"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"G", "F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"F", "G"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"I"}),
					},
					{
						conflictAlias:     "I",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"I"}),
					},
					{
						conflictAlias:     "J",
						wantLikedConflict: mustMatch(&scenario, []string{"K"}, []string{"J"}),
					},
					{
						conflictAlias:     "K",
						wantLikedConflict: mustMatch(&scenario, []string{"K"}, []string{"J"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "H", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"H", "C"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"F", "G", "H"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"F", "G", "H"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"C", "F", "G", "H"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"H", "C", "B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"H", "B"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{"G"}, []string{"H", "F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{"G"}, []string{"F", "H"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"C"}, []string{"B", "F", "G", "H"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"H", "C", "B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"G", "F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"F", "G"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"F", "G", "B", "C"}),
					},
					{
						conflictAlias:     "I",
						wantLikedConflict: mustMatch(&scenario, []string{"J"}, []string{"O", "I"}),
					},
					{
						conflictAlias:     "J",
						wantLikedConflict: mustMatch(&scenario, []string{"J"}, []string{"I", "O"}),
					},
					{
						conflictAlias:     "K",
						wantLikedConflict: mustMatch(&scenario, []string{"L"}, []string{"K"}),
					},
					{
						conflictAlias:     "L",
						wantLikedConflict: mustMatch(&scenario, []string{"L"}, []string{"K"}),
					},
					{
						conflictAlias:     "M",
						wantLikedConflict: mustMatch(&scenario, []string{"N"}, []string{"O", "M"}),
					},
					{
						conflictAlias:     "N",
						wantLikedConflict: mustMatch(&scenario, []string{"N"}, []string{"M", "O"}),
					},
					{
						conflictAlias:     "O",
						wantLikedConflict: mustMatch(&scenario, []string{"J"}, []string{"M", "N", "I", "O"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"B"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"A"}, []string{"H", "C", "B"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"B", "C"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"G", "F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"F", "G"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"H"}, []string{"F", "G", "B", "C"}),
					},
					{
						conflictAlias:     "I",
						wantLikedConflict: mustMatch(&scenario, []string{"O"}, []string{"J", "I"}),
					},
					{
						conflictAlias:     "J",
						wantLikedConflict: mustMatch(&scenario, []string{"O"}, []string{"I", "J"}),
					},
					{
						conflictAlias:     "K",
						wantLikedConflict: mustMatch(&scenario, []string{"L"}, []string{"K"}),
					},
					{
						conflictAlias:     "L",
						wantLikedConflict: mustMatch(&scenario, []string{"L"}, []string{"K"}),
					},
					{
						conflictAlias:     "M",
						wantLikedConflict: mustMatch(&scenario, []string{"O"}, []string{"N", "M"}),
					},
					{
						conflictAlias:     "N",
						wantLikedConflict: mustMatch(&scenario, []string{"O"}, []string{"M", "N"}),
					},
					{
						conflictAlias:     "O",
						wantLikedConflict: mustMatch(&scenario, []string{"O"}, []string{"M", "N", "J", "I"}),
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
						conflictAlias:     "A",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A"}),
					},
					{
						conflictAlias:     "B",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"A", "H", "C"}),
					},
					{
						conflictAlias:     "C",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"H", "C"}),
					},
					{
						conflictAlias:     "F",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"G", "H", "F"}),
					},
					{
						conflictAlias:     "G",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"F", "H", "G"}),
					},
					{
						conflictAlias:     "H",
						wantLikedConflict: mustMatch(&scenario, []string{"B"}, []string{"C", "F", "G", "H"}),
					},
					{
						conflictAlias:     "I",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"J", "I"}),
					},
					{
						conflictAlias:     "J",
						wantLikedConflict: mustMatch(&scenario, []string{}, []string{"I", "J"}),
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

			tt.test.Scenario.CreateConflicts(t, ls.ConflictDAG)
			o := NewOnTangleVoting(ls.ConflictDAG, tt.test.WeightFunc)

			for _, e := range tt.test.executions {
				liked, conflictMembers := o.LikedConflictMember(tt.test.Scenario.ConflictID(e.conflictAlias))
				e.wantLikedConflict(e.conflictAlias, liked, conflictMembers)
			}
		})
	}
}

// region test helpers /////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMeta describes a conflict in a conflictDAG with its conflicts and approval weight.
type ConflictMeta struct {
	Order           int
	ConflictID      utxo.TransactionID
	ParentConflicts *set.AdvancedSet[utxo.TransactionID]
	Conflicting     *set.AdvancedSet[utxo.OutputID]
	ApprovalWeight  float64
}

// Scenario is a testing utility representing a conflictDAG with additional information such as approval weight for each
// individual conflict.
type Scenario map[string]*ConflictMeta

// IDsToNames returns a mapping of ConflictIDs to their alias.
func (s *Scenario) IDsToNames() map[utxo.TransactionID]string {
	mapping := map[utxo.TransactionID]string{}
	for name, m := range *s {
		mapping[m.ConflictID] = name
	}
	return mapping
}

// ConflictID returns the ConflictID of the given conflict alias.
func (s *Scenario) ConflictID(alias string) utxo.TransactionID {
	return (*s)[alias].ConflictID
}

// ConflictIDs returns either all ConflictIDs in the scenario or only the ones with the given aliases.
func (s *Scenario) ConflictIDs(aliases ...string) *set.AdvancedSet[utxo.TransactionID] {
	conflictIDs := set.NewAdvancedSet[utxo.TransactionID]()
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
		conflictIDs.Add(meta.ConflictID)
	}
	return conflictIDs
}

// CreateConflicts orders and creates the conflicts for the scenario.
func (s *Scenario) CreateConflicts(t *testing.T, conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) {
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
		createTestConflict(t, conflictDAG, o.name, m)
	}
}

// creates a conflict and registers a ConflictIDAlias with the name specified in conflictMeta.
func createTestConflict(t *testing.T, conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], alias string, conflictMeta *ConflictMeta) bool {
	var newConflictCreated bool

	if conflictMeta.ConflictID == utxo.EmptyTransactionID {
		panic("a conflict must have its ID defined in its ConflictMeta")
	}
	newConflictCreated = conflictDAG.CreateConflict(conflictMeta.ConflictID, conflictMeta.ParentConflicts, conflictMeta.Conflicting)
	require.True(t, newConflictCreated)
	conflictDAG.Storage.CachedConflict(conflictMeta.ConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		conflictMeta.ConflictID = conflict.ID()
	})
	conflictMeta.ConflictID.RegisterAlias(alias)
	return newConflictCreated
}

// WeightFuncFromScenario creates a WeightFunc from the given scenario so that the approval weight can be mocked
// according to the conflict weight's specified in the scenario.
func WeightFuncFromScenario(t *testing.T, scenario Scenario) consensus.WeightFunc {
	conflictIDsToName := scenario.IDsToNames()
	return func(conflictID utxo.TransactionID) (weight float64) {
		name, nameOk := conflictIDsToName[conflictID]
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
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.6,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
	}

	s2 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.6,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.8,
		},
	}

	s3 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.5,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.4,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
	}

	s4 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.3,
		},
	}

	s45 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{200}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.3,
		},
	}

	s5 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.1,
		},
	}

	s6 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID5),
			ApprovalWeight:  0.4,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID5),
			ApprovalWeight:  0.1,
		},
	}

	s7 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.1,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.15,
		},
	}

	s8 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.1,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0),
			ApprovalWeight:  0.5,
		},
	}

	s9 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.1,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0),
			ApprovalWeight:  0.1,
		},
	}

	s10 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID1),
			ApprovalWeight:  0.1,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID0, conflictID2),
			ApprovalWeight:  0.3,
		},
	}

	s12 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.25,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.35,
		},
	}

	s13 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.4,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.35,
		},
	}

	s14 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.4,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.35,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.17,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  0.1,
		},
		"I": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  0.05,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  0.04,
		},
		"K": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  0.06,
		},
	}

	s15 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
		"D": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{5}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.15,
		},
		"E": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{6}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID3),
			ApprovalWeight:  0.35,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.17,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  0.1,
		},
		"I": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{6}}),
			Conflicting:     set.NewAdvancedSet(conflictID6),
			ApprovalWeight:  0.05,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  0.04,
		},
		"K": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID9),
			ApprovalWeight:  0.06,
		},
	}

	s16 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.03,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  0.15,
		},
	}

	s17 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.1,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.03,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  0.15,
		},
	}

	s18 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.1,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.05,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.03,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  0.15,
		},
		"K": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  0.1,
		},
		"L": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  0.2,
		},
		"M": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  0.05,
		},
		"N": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  0.06,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  0.07,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  0.08,
		},
		"O": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight:  0.05,
		},
	}

	s19 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.3,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.1,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.05,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.03,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  0.15,
		},
		"K": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  0.1,
		},
		"L": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID11),
			ApprovalWeight:  0.2,
		},
		"M": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{12}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  0.05,
		},
		"N": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{13}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID12),
			ApprovalWeight:  0.06,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{14}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  0.07,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{15}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}),
			Conflicting:     set.NewAdvancedSet(conflictID8),
			ApprovalWeight:  0.08,
		},
		"O": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{16}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{9}}, utxo.TransactionID{Identifier: types.Identifier{11}}),
			Conflicting:     set.NewAdvancedSet(conflictID8, conflictID12),
			ApprovalWeight:  0.09,
		},
	}

	s20 = Scenario{
		"A": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{2}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1),
			ApprovalWeight:  0.2,
		},
		"B": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{3}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID1, conflictID2),
			ApprovalWeight:  0.3,
		},
		"C": {
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{4}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID),
			Conflicting:     set.NewAdvancedSet(conflictID2),
			ApprovalWeight:  0.2,
		},
		"F": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{7}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.02,
		},
		"G": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{8}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID4),
			ApprovalWeight:  0.03,
		},
		"H": {
			Order:           1,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{9}},
			ParentConflicts: set.NewAdvancedSet(utxo.EmptyTransactionID, utxo.TransactionID{Identifier: types.Identifier{2}}),
			Conflicting:     set.NewAdvancedSet(conflictID2, conflictID4),
			ApprovalWeight:  0.15,
		},
		"I": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{10}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:     set.NewAdvancedSet(conflictID7),
			ApprovalWeight:  0.005,
		},
		"J": {
			Order:           2,
			ConflictID:      utxo.TransactionID{Identifier: types.Identifier{11}},
			ParentConflicts: set.NewAdvancedSet(utxo.TransactionID{Identifier: types.Identifier{7}}),
			Conflicting:     set.NewAdvancedSet(conflictID7),
			ApprovalWeight:  0.015,
		},
	}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
