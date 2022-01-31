//nolint:dupl
package otv

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/goshimmer/packages/consensus"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
	. "github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestOnTangleVoting_LikedInstead(t *testing.T) {
	type ExpectedOpinionTuple func(executionBranchAlias string, branchIDs []consensus.OpinionTuple)

	sortOpinionTuple := func(ot []consensus.OpinionTuple) {
		sort.Slice(ot, func(x, y int) bool {
			switch bytes.Compare(ot[x].Liked.Bytes(), ot[y].Liked.Bytes()) {
			case -1:
				return true
			case 1:
				return false
			default:
				return bytes.Compare(ot[x].Disliked.Bytes(), ot[y].Disliked.Bytes()) <= 0
			}
		})
	}

	mustMatch := func(s *Scenario, aliasTuples ...aliasOpinionTuple) ExpectedOpinionTuple {
		return func(executionBranchAlias string, actual []consensus.OpinionTuple) {
			expected := createOpinionTuples(s, aliasTuples...)
			sortOpinionTuple(expected)
			sortOpinionTuple(actual)
			if assert.EqualValues(t, expected, actual) {
				return
			}
			fmt.Printf("failed execution with Branch '%s'\n", executionBranchAlias)
			fmt.Println("expected", expected)
			fmt.Println("actual", actual)
			t.FailNow()
		}
	}

	type execution struct {
		branchAlias      string
		wantOpinionTuple ExpectedOpinionTuple
		wantErr          bool
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
						),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"C", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
						),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "C"},
						),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
						),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "C"},
						),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"C", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"C", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"C", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "D"},
						),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"D", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"D", "C"},
						),
					},
					{
						branchAlias:      "D",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"E", "B"},
						),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "C"},
						),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "D"},
						),
					},
					{
						branchAlias:      "E",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"D", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"D", "C"},
						),
					},
					{
						branchAlias:      "D",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "E",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "E"},
						),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"C", "A"},
						),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"C", "B"},
						),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"E", "D"},
						),
					},
					{
						branchAlias:      "E",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C+E",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"C", "B"},
						),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"E", "D"},
						),
					},
					{
						branchAlias:      "E",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C+E",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"C", "B"},
						),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"E", "D"},
						),
					},
					{
						branchAlias:      "E",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"G", "F"},
						),
					},
					{
						branchAlias:      "G",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "C+E",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias:      "H",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "I",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "I"},
						),
					},
					{
						branchAlias:      "C+E+G",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "J",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"K", "J"},
						),
					},
					{
						branchAlias:      "K",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias: "D",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"E", "D"},
						),
					},
					{
						branchAlias:      "E",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "C+E",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias:      "H",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "I",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "I"},
						),
					},
					{
						branchAlias: "C+E+G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias: "J",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"K", "J"},
						),
					},
					{
						branchAlias:      "K",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "H",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"B", "H"},
						),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"C", "B"},
						),
					},
					{
						branchAlias:      "C",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"G", "F"},
						),
					},
					{
						branchAlias:      "G",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "H",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"G", "H"},
							aliasOpinionTuple{"C", "H"},
						),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"H", "B"},
						),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "C"},
						),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "F"},
						),
					},
					{
						branchAlias: "G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "G"},
						),
					},
					{
						branchAlias:      "H",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "I",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"J", "I"},
						),
					},
					{
						branchAlias:      "J",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "K",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"L", "K"},
						),
					},
					{
						branchAlias:      "L",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "M",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"N", "M"},
						),
					},
					{
						branchAlias:      "N",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "O",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"J", "O"},
							aliasOpinionTuple{"N", "O"},
						),
					},
					{
						branchAlias:      "J+N",
						wantOpinionTuple: mustMatch(&scenario),
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
						branchAlias:      "A",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "B",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"A", "B"},
							aliasOpinionTuple{"H", "B"},
						),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "C"},
						),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "F"},
						),
					},
					{
						branchAlias: "G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"H", "G"},
						),
					},
					{
						branchAlias:      "H",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "I",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"O", "I"},
						),
					},
					{
						branchAlias: "J",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"O", "J"},
						),
					},
					{
						branchAlias: "K",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"L", "K"},
						),
					},
					{
						branchAlias:      "L",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "M",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"O", "M"},
						),
					},
					{
						branchAlias: "N",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"O", "N"},
						),
					},
					{
						branchAlias:      "O",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "J+N",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"O", "J"},
							aliasOpinionTuple{"O", "N"},
						),
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
						branchAlias: "A",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias:      "B",
						wantOpinionTuple: mustMatch(&scenario),
					},
					{
						branchAlias: "C",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "C"},
						),
					},
					{
						branchAlias: "F",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "G",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "H",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
							aliasOpinionTuple{"B", "H"},
						),
					},
					{
						branchAlias: "I",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
					},
					{
						branchAlias: "J",
						wantOpinionTuple: mustMatch(&scenario,
							aliasOpinionTuple{"B", "A"},
						),
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
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			defer branchDAG.Shutdown()

			tt.test.Scenario.CreateBranches(t, branchDAG)
			o := NewOnTangleVoting(branchDAG, tt.test.WeightFunc)

			for _, e := range tt.test.executions {
				liked, err := o.LikedInstead(tt.test.Scenario.BranchID(e.branchAlias))
				if e.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				e.wantOpinionTuple(e.branchAlias, liked)
			}
		})
	}
}

func TestOnTangleVoting_Opinion(t *testing.T) {
	type ExpectedBranchesFunc func(branchIDs BranchIDs)
	type ArgsFunc func() (branchIDs BranchIDs)

	mustMatch := func(s *Scenario, aliases ...string) ExpectedBranchesFunc {
		return func(actual BranchIDs) {
			if !assert.EqualValues(t, s.BranchIDs(aliases...), actual) {
				fmt.Println("expected", s.BranchIDs(aliases...))
				fmt.Println("actual", actual)
			}
		}
	}

	argsFunc := func(s *Scenario, aliases ...string) ArgsFunc {
		return func() (branchIDs BranchIDs) {
			return s.BranchIDs(aliases...)
		}
	}

	type test struct {
		Scenario     Scenario
		WeightFunc   consensus.WeightFunc
		args         ArgsFunc
		wantLiked    ExpectedBranchesFunc
		wantDisliked ExpectedBranchesFunc
	}

	tests := []struct {
		name    string
		test    test
		wantErr bool
	}{
		{
			name: "1",
			test: func() test {
				scenario := s1

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "2",
			test: func() test {
				scenario := s2

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "3",
			test: func() test {
				scenario := s3

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B", "C"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "4",
			test: func() test {
				scenario := s4

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B", "C"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "4.5",
			test: func() test {
				scenario := s45

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "5",
			test: func() test {
				scenario := s5

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "6",
			test: func() test {
				scenario := s6

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A", "D"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "7",
			test: func() test {
				scenario := s7

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "D"),
					wantDisliked: mustMatch(&scenario, "A", "C"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "8",
			test: func() test {
				scenario := s8

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "E"),
					wantDisliked: mustMatch(&scenario, "B", "C", "D"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "9",
			test: func() test {
				scenario := s9

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "D"),
					wantDisliked: mustMatch(&scenario, "A", "C", "E"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "10",
			test: func() test {
				scenario := s10

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "C"),
					wantDisliked: mustMatch(&scenario, "A", "B"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "12",
			test: func() test {
				scenario := s12

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "E"),
					wantDisliked: mustMatch(&scenario, "A", "C", "D"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "13",
			test: func() test {
				scenario := s13

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "E", "C+E"),
					wantDisliked: mustMatch(&scenario, "B", "D"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "14",
			test: func() test {
				scenario := s14

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "E", "G", "H", "K", "C+E", "C+E+G"),
					wantDisliked: mustMatch(&scenario, "B", "D", "F", "J", "I"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "15",
			test: func() test {
				scenario := s15

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "E", "H", "K"),
					wantDisliked: mustMatch(&scenario, "A", "C", "D", "I", "J"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "16",
			test: func() test {
				scenario := s16

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B"),
					wantDisliked: mustMatch(&scenario, "A", "C", "H"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "17",
			test: func() test {
				scenario := s17

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "G"),
					wantDisliked: mustMatch(&scenario, "B", "F", "H"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "18",
			test: func() test {
				scenario := s18

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "H", "L", "N", "J", "J+N"),
					wantDisliked: mustMatch(&scenario, "B", "C", "F", "G", "I", "K", "M", "O"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "19",
			test: func() test {
				scenario := s19

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "H", "L", "O"),
					wantDisliked: mustMatch(&scenario, "B", "C", "F", "G", "I", "N", "K", "M", "J"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "20",
			test: func() test {
				scenario := s20

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B"),
					wantDisliked: mustMatch(&scenario, "A", "C", "H"),
					args:         argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			defer branchDAG.Shutdown()

			tt.test.Scenario.CreateBranches(t, branchDAG)
			o := NewOnTangleVoting(branchDAG, tt.test.WeightFunc)

			gotLiked, gotDisliked, err := o.Opinion(tt.test.args())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			tt.test.wantLiked(gotLiked)
			tt.test.wantDisliked(gotDisliked)
		})
	}
}

// region test helpers /////////////////////////////////////////////////////////////////////////////////////////////////

// aliasOpinionTuple allows to specify consensus.OpinionTuple with branch aliases.
type aliasOpinionTuple struct {
	Liked    string
	Disliked string
}

// createOpinionTuples creates a slice of consensus.OpinionTuple from aliasOpinionTuple.
func createOpinionTuples(scenario *Scenario, aliasTuples ...aliasOpinionTuple) []consensus.OpinionTuple {
	ots := make([]consensus.OpinionTuple, 0)
	for _, a := range aliasTuples {
		ots = append(ots, consensus.OpinionTuple{
			Liked:    scenario.BranchID(a.Liked),
			Disliked: scenario.BranchID(a.Disliked),
		})
	}

	return ots
}

// BranchMeta describes a branch in a branchDAG with its conflicts and approval weight.
type BranchMeta struct {
	Order          int
	BranchID       BranchID
	ParentBranches BranchIDs
	Conflicting    ConflictIDs
	ApprovalWeight float64
	IsAggregated   bool
}

// Scenario is a testing utility representing a branchDAG with additional information such as approval weight for each
// individual branch.
type Scenario map[string]*BranchMeta

// IDsToNames returns a mapping of BranchIDs to their alias.
func (s *Scenario) IDsToNames() map[BranchID]string {
	mapping := map[BranchID]string{}
	for name, m := range *s {
		mapping[m.BranchID] = name
	}
	return mapping
}

// BranchID returns the BranchID of the given branch alias.
func (s *Scenario) BranchID(alias string) BranchID {
	return (*s)[alias].BranchID
}

// BranchIDs returns either all BranchIDs in the scenario or only the ones with the given aliases.
func (s *Scenario) BranchIDs(aliases ...string) BranchIDs {
	branchIDs := NewBranchIDs()
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
func (s *Scenario) CreateBranches(t *testing.T, branchDAG *BranchDAG) {
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
		createTestBranch(t, branchDAG, o.name, m, m.IsAggregated)
	}
}

// creates a branch and registers a BranchIDAlias with the name specified in branchMeta.
func createTestBranch(t *testing.T, branchDAG *BranchDAG, alias string, branchMeta *BranchMeta, isAggregated bool) bool {
	var cachedBranch *CachedBranch
	var newBranchCreated bool
	var err error
	if isAggregated {
		if len(branchMeta.ParentBranches) == 0 {
			panic("an aggregated branch must have parents defined")
		}
		cachedBranch, newBranchCreated, err = branchDAG.AggregateBranches(branchMeta.ParentBranches)
	} else {
		if branchMeta.BranchID == UndefinedBranchID {
			panic("a non aggr. branch must have its ID defined in its BranchMeta")
		}
		cachedBranch, newBranchCreated, err = branchDAG.CreateConflictBranch(branchMeta.BranchID, branchMeta.ParentBranches, branchMeta.Conflicting)
	}
	require.NoError(t, err)
	require.True(t, newBranchCreated)
	defer cachedBranch.Release()
	branchMeta.BranchID = cachedBranch.ID()
	RegisterBranchIDAlias(branchMeta.BranchID, alias)
	return newBranchCreated
}

// WeightFuncFromScenario creates a WeightFunc from the given scenario so that the approval weight can be mocked
// according to the branch weight's specified in the scenario.
func WeightFuncFromScenario(t *testing.T, scenario Scenario) consensus.WeightFunc {
	branchIDsToName := scenario.IDsToNames()
	return func(branchID BranchID) (weight float64) {
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
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.6,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
	}

	s2 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.6,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.8,
		},
	}

	s3 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.5,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
	}

	s4 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.3,
		},
	}

	s45 = Scenario{
		"A": {
			BranchID:       BranchID{200},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.3,
		},
	}

	s5 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.1,
		},
	}

	s6 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{7}),
			ApprovalWeight: 0.4,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{7}),
			ApprovalWeight: 0.1,
		},
	}

	s7 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.15,
		},
	}

	s8 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}),
			ApprovalWeight: 0.5,
		},
	}

	s9 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}),
			ApprovalWeight: 0.1,
		},
	}

	s10 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
	}

	s12 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.25,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.35,
		},
		"C+E": {
			Order:          1,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: 0.35,
			IsAggregated:   true,
		},
	}

	s13 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.35,
		},
		"C+E": {
			Order:          1,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: 0.35,
			IsAggregated:   true,
		},
	}

	s14 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.4,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.35,
		},
		"C+E": {
			Order:          2,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
			IsAggregated:   true,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.17,
		},
		"C+E+G": {
			Order:          2,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}, BranchID{8}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
			IsAggregated:   true,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(BranchID{6}),
			Conflicting:    NewConflictIDs(ConflictID{10}),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       BranchID{10},
			ParentBranches: NewBranchIDs(BranchID{6}),
			Conflicting:    NewConflictIDs(ConflictID{10}),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       BranchID{11},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{15}),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       BranchID{12},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{15}),
			ApprovalWeight: 0.06,
		},
	}

	s15 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"D": {
			BranchID:       BranchID{5},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.15,
		},
		"E": {
			BranchID:       BranchID{6},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{3}),
			ApprovalWeight: 0.35,
		},
		"C+E": {
			Order:          2,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
			IsAggregated:   true,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.17,
		},
		"C+E+G": {
			Order:          2,
			ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}, BranchID{8}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
			IsAggregated:   true,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(BranchID{6}),
			Conflicting:    NewConflictIDs(ConflictID{10}),
			ApprovalWeight: 0.1,
		},
		"I": {
			Order:          1,
			BranchID:       BranchID{10},
			ParentBranches: NewBranchIDs(BranchID{6}),
			Conflicting:    NewConflictIDs(ConflictID{10}),
			ApprovalWeight: 0.05,
		},
		"J": {
			Order:          2,
			BranchID:       BranchID{11},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{15}),
			ApprovalWeight: 0.04,
		},
		"K": {
			Order:          2,
			BranchID:       BranchID{12},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{15}),
			ApprovalWeight: 0.06,
		},
	}

	s16 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(MasterBranchID, BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{2}, ConflictID{4}),
			ApprovalWeight: 0.15,
		},
	}

	s17 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(MasterBranchID, BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{2}, ConflictID{4}),
			ApprovalWeight: 0.15,
		},
	}

	s18 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(MasterBranchID, BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{2}, ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       BranchID{10},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{17}),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       BranchID{11},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{17}),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       BranchID{12},
			ParentBranches: NewBranchIDs(BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{19}),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       BranchID{13},
			ParentBranches: NewBranchIDs(BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{19}),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       BranchID{14},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{14}),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       BranchID{15},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{14}),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       BranchID{16},
			ParentBranches: NewBranchIDs(BranchID{9}, BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{14}, ConflictID{19}),
			ApprovalWeight: 0.05,
		},
		"J+N": {
			Order:          3,
			IsAggregated:   true,
			ParentBranches: NewBranchIDs(BranchID{15}, BranchID{13}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
		},
	}

	s19 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.3,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.1,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.05,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(MasterBranchID, BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{2}, ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"K": {
			BranchID:       BranchID{10},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{17}),
			ApprovalWeight: 0.1,
		},
		"L": {
			BranchID:       BranchID{11},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{17}),
			ApprovalWeight: 0.2,
		},
		"M": {
			Order:          1,
			BranchID:       BranchID{12},
			ParentBranches: NewBranchIDs(BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{19}),
			ApprovalWeight: 0.05,
		},
		"N": {
			Order:          1,
			BranchID:       BranchID{13},
			ParentBranches: NewBranchIDs(BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{19}),
			ApprovalWeight: 0.06,
		},
		"I": {
			Order:          2,
			BranchID:       BranchID{14},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{14}),
			ApprovalWeight: 0.07,
		},
		"J": {
			Order:          2,
			BranchID:       BranchID{15},
			ParentBranches: NewBranchIDs(BranchID{9}),
			Conflicting:    NewConflictIDs(ConflictID{14}),
			ApprovalWeight: 0.08,
		},
		"O": {
			Order:          2,
			BranchID:       BranchID{16},
			ParentBranches: NewBranchIDs(BranchID{9}, BranchID{11}),
			Conflicting:    NewConflictIDs(ConflictID{14}, ConflictID{19}),
			ApprovalWeight: 0.09,
		},
		"J+N": {
			Order:          3,
			IsAggregated:   true,
			ParentBranches: NewBranchIDs(BranchID{15}, BranchID{13}),
			Conflicting:    NewConflictIDs(),
			ApprovalWeight: -1,
		},
	}

	s20 = Scenario{
		"A": {
			BranchID:       BranchID{2},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}),
			ApprovalWeight: 0.2,
		},
		"B": {
			BranchID:       BranchID{3},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
			ApprovalWeight: 0.3,
		},
		"C": {
			BranchID:       BranchID{4},
			ParentBranches: NewBranchIDs(MasterBranchID),
			Conflicting:    NewConflictIDs(ConflictID{2}),
			ApprovalWeight: 0.2,
		},
		"F": {
			Order:          1,
			BranchID:       BranchID{7},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.02,
		},
		"G": {
			Order:          1,
			BranchID:       BranchID{8},
			ParentBranches: NewBranchIDs(BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{4}),
			ApprovalWeight: 0.03,
		},
		"H": {
			Order:          1,
			BranchID:       BranchID{9},
			ParentBranches: NewBranchIDs(MasterBranchID, BranchID{2}),
			Conflicting:    NewConflictIDs(ConflictID{2}, ConflictID{4}),
			ApprovalWeight: 0.15,
		},
		"I": {
			Order:          2,
			BranchID:       BranchID{10},
			ParentBranches: NewBranchIDs(BranchID{7}),
			Conflicting:    NewConflictIDs(ConflictID{12}),
			ApprovalWeight: 0.005,
		},
		"J": {
			Order:          2,
			BranchID:       BranchID{11},
			ParentBranches: NewBranchIDs(BranchID{7}),
			Conflicting:    NewConflictIDs(ConflictID{12}),
			ApprovalWeight: 0.015,
		},
	}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
