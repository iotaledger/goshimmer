package otv

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
	. "github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type BranchMeta struct {
	BranchID       BranchID
	ParentBranches BranchIDs
	Conflicting    ConflictIDs
	ApprovalWeight float64
	IsAggregated   bool
}

func (bm *BranchMeta) ToConflictMembers() []*ConflictMember {
	m := make([]*ConflictMember, 0)
	for conflictID := range bm.Conflicting {
		m = append(m, NewConflictMember(conflictID, bm.BranchID))
	}
	return m
}

type Scenario map[string]*BranchMeta

// IDsToNames returns a mapping of BranchIDs to their alias.
func (s *Scenario) IDsToNames() map[BranchID]string {
	mapping := map[BranchID]string{}
	for name, m := range *s {
		mapping[m.BranchID] = name
	}
	return mapping
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

func createTestBranch(t *testing.T, branchDAG *BranchDAG, alias string, branchMeta *BranchMeta, isAggregated bool) bool {
	var cachedBranch *CachedBranch
	var newBranchCreated bool
	var err error
	if isAggregated {
		if len(branchMeta.ParentBranches) == 0 {
			panic("an aggregated branch must have parents defined")
		}
		cachedBranch, newBranchCreated, err = branchDAG.AggregateBranches(branchMeta.ParentBranches)
		branchMeta.BranchID = cachedBranch.ID()
	} else {
		emptyBranch := BranchID{}
		if branchMeta.BranchID == emptyBranch {
			panic("a non aggr. branch must have its ID defined in its BranchMeta")
		}
		cachedBranch, newBranchCreated, err = branchDAG.CreateConflictBranch(branchMeta.BranchID, branchMeta.ParentBranches, branchMeta.Conflicting)
	}
	require.NoError(t, err)
	require.True(t, newBranchCreated)
	defer cachedBranch.Release()
	RegisterBranchIDAlias(branchMeta.BranchID, alias)
	return newBranchCreated
}

func WeightFuncFromScenario(t *testing.T, scenario Scenario) WeightFunc {
	branchIDsToName := scenario.IDsToNames()
	return func(branchID BranchID) (weight float64) {
		name, nameOk := branchIDsToName[branchID]
		require.True(t, nameOk)
		meta, metaOk := scenario[name]
		require.True(t, metaOk)
		return meta.ApprovalWeight
	}
}
func TestLikedFromConflictSet(t *testing.T) {

	type ExpectedBranchFunc func(branchID BranchID)

	mustMatch := func(expected BranchID) ExpectedBranchFunc {
		return func(actual BranchID) {
			require.Equal(t, expected, actual)
		}
	}
	type test struct {
		Scenario   Scenario
		WeightFunc WeightFunc
		args       BranchID
		wantLiked  ExpectedBranchFunc
	}

	tests := []struct {
		name     string
		test     test
		scenario Scenario
		wantErr  bool
	}{
		{
			name: "1",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{2}),
					args:       BranchID{3},
				}
			}(),
			wantErr: false,
		},
		{
			name: "2",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{4}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "3",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{2}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "4",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{2}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "4.5",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{4}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "5",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{4}),
					args:       BranchID{2},
				}
			}(),
			wantErr: false,
		},
		{
			name: "6",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{3}),
					args:       BranchID{3},
				}
			}(),
			wantErr: false,
		},
		{
			name: "7",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{3}),
					args:       BranchID{2},
				}
			}(),
			wantErr: false,
		},
		{
			name: "8",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{2}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "9",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{5}),
					args:       BranchID{4},
				}
			}(),
			wantErr: false,
		},
		{
			name: "10",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					wantLiked:  mustMatch(BranchID{4}),
					args:       BranchID{2},
				}
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			for name, m := range tt.test.Scenario {
				createTestBranch(t, branchDAG, name, m, m.IsAggregated)
			}
			o := &OnTangleVoting{branchDAG: branchDAG, weightFunc: tt.test.WeightFunc}

			liked, err := o.LikedFromConflictSet(tt.test.args)
			require.NoError(t, err)
			tt.test.wantLiked(liked)
		})
	}
}
func TestDoILike(t *testing.T) {
	type test struct {
		Scenario   Scenario
		WeightFunc WeightFunc
		args       []*ConflictMember
		wanted     []*ConflictMember
	}

	tests := []struct {
		name     string
		test     test
		scenario Scenario
		wantErr  bool
	}{
		{
			name: "pair-wise conflicting (inverse)",
			test: func() test {
				scenario := Scenario{
					"A": {
						BranchID:       BranchID{2},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{2}),
						ApprovalWeight: 0.3,
					},
					"B": {
						BranchID:       BranchID{3},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{1}, ConflictID{5}),
						ApprovalWeight: 0.4,
					},
					"C": {
						BranchID:       BranchID{4},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{2}),
						ApprovalWeight: 0.2,
					},
					"E": {
						BranchID:       BranchID{5},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{5}),
						ApprovalWeight: 0.1,
					},
				}

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					args: []*ConflictMember{
						NewConflictMember(ConflictID{2}, BranchID{2}),
						NewConflictMember(ConflictID{2}, BranchID{4}),
					},
					wanted: []*ConflictMember{
						NewConflictMember(ConflictID{2}, BranchID{4}),
					},
				}
			}(),
			wantErr: false,
		},
		{
			name: "pair-wise conflicting (inverse)",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:   scenario,
					WeightFunc: WeightFuncFromScenario(t, scenario),
					args: []*ConflictMember{
						NewConflictMember(ConflictID{2}, BranchID{2}),
						NewConflictMember(ConflictID{2}, BranchID{4}),
					},
					wanted: []*ConflictMember{
						NewConflictMember(ConflictID{2}, BranchID{4}),
					},
				}
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			for name, m := range tt.test.Scenario {
				createTestBranch(t, branchDAG, name, m, m.IsAggregated)
			}
			o := &OnTangleVoting{branchDAG: branchDAG, weightFunc: tt.test.WeightFunc}

			//filtered := o.resolve(tt.test.args)
			liked := o.doILike(BranchID{5}, NewConflictIDs())
			fmt.Println(liked)
			//require.EqualValues(t, tt.test.wanted, filtered)
		})
	}
}

func TestOnTangleVoting_Opinion(t *testing.T) {
	type ExpectedBranchesFunc func(branchIDs BranchIDs)
	type ExpectedBranchesPairFunc func(liked, disliked BranchIDs)

	mustMatch := func(expected BranchIDs) ExpectedBranchesFunc {
		return func(actual BranchIDs) {
			require.EqualValues(t, expected, actual)
		}
	}

	type test struct {
		Scenario     Scenario
		WeightFunc   WeightFunc
		args         BranchIDs
		wantLiked    ExpectedBranchesFunc
		wantDisliked ExpectedBranchesFunc
		wanted       ExpectedBranchesPairFunc
	}

	tests := []struct {
		name    string
		test    test
		wantErr bool
	}{
		{
			name: "1",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("A")),
					wantDisliked: mustMatch(scenario.BranchIDs("B")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "2",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "C")),
					wantDisliked: mustMatch(scenario.BranchIDs("A")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "3",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("A")),
					wantDisliked: mustMatch(scenario.BranchIDs("B", "C")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "4",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("A")),
					wantDisliked: mustMatch(scenario.BranchIDs("B", "C")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "4.5",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "C")),
					wantDisliked: mustMatch(scenario.BranchIDs("A")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "5",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "C")),
					wantDisliked: mustMatch(scenario.BranchIDs("A")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "6",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "C")),
					wantDisliked: mustMatch(scenario.BranchIDs("A", "D")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "7",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "D")),
					wantDisliked: mustMatch(scenario.BranchIDs("A", "C")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "8",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("A", "E")),
					wantDisliked: mustMatch(scenario.BranchIDs("B", "C", "D")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "9",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("B", "D")),
					wantDisliked: mustMatch(scenario.BranchIDs("A", "C", "E")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "10",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("C")),
					wantDisliked: mustMatch(scenario.BranchIDs("A", "B")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "12",
			test: func() test {
				scenario := Scenario{
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
						ParentBranches: NewBranchIDs(BranchID{5}, BranchID{6}),
						Conflicting:    NewConflictIDs(),
						ApprovalWeight: 0.35,
						IsAggregated:   true,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(scenario.BranchIDs("C+E")),
					wantDisliked: mustMatch(scenario.BranchIDs("A", "B")),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			// TODO: make sure that all objects are properly released
			// defer branchDAG.Shutdown()

			for name, m := range tt.test.Scenario {
				createTestBranch(t, branchDAG, name, m, m.IsAggregated)
			}
			o := &OnTangleVoting{branchDAG: branchDAG, weightFunc: tt.test.WeightFunc}

			gotLiked, gotDisliked, err := o.Opinion(tt.test.args)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.test.wanted != nil {
				tt.test.wanted(gotLiked, gotDisliked)
				return
			}
			tt.test.wantLiked(gotLiked)
			tt.test.wantDisliked(gotDisliked)
		})
	}
}
