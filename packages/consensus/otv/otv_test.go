package otv

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/database"
	. "github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/require"
)

type BranchMeta struct {
	BranchID       BranchID
	ParentBranches BranchIDs
	Conflicting    ConflictIDs
	ApprovalWeight float64
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

func createTestBranch(t *testing.T, branchDAG *BranchDAG, alias string, branchMeta *BranchMeta) bool {
	cachedConflictBranch, newBranchCreated, err := branchDAG.CreateConflictBranch(branchMeta.BranchID, branchMeta.ParentBranches, branchMeta.Conflicting)
	require.NoError(t, err)
	require.True(t, newBranchCreated)
	defer cachedConflictBranch.Release()
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

func TestOnTangleVoting_Opinion(t *testing.T) {

	type test struct {
		Scenario     Scenario
		WeightFunc   WeightFunc
		args         BranchIDs
		wantLiked    BranchIDs
		wantDisliked BranchIDs
	}
	tests := []struct {
		name     string
		test     test
		scenario Scenario
		wantErr  bool
	}{
		{
			name: "simple",
			test: func() test {
				scenario := Scenario{
					"A": {
						BranchID:       BranchID{2},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}),
						ApprovalWeight: 0.6,
					},
					"B": {
						BranchID:       BranchID{3},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}),
						ApprovalWeight: 0.3,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    scenario.BranchIDs("A"),
					wantDisliked: scenario.BranchIDs("B"),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
		{
			name: "pair-wise conflicting",
			test: func() test {
				scenario := Scenario{
					"A": {
						BranchID:       BranchID{2},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
						ApprovalWeight: 0.2,
					},
					"B": {
						BranchID:       BranchID{3},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}),
						ApprovalWeight: 0.6,
					},
					"C": {
						BranchID:       BranchID{4},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{1}),
						ApprovalWeight: 0.8,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    scenario.BranchIDs("B", "C"),
					wantDisliked: scenario.BranchIDs("A"),
					args:         scenario.BranchIDs(),
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
						Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
						ApprovalWeight: 0.8,
					},
					"B": {
						BranchID:       BranchID{3},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}),
						ApprovalWeight: 0.6,
					},
					"C": {
						BranchID:       BranchID{4},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{1}),
						ApprovalWeight: 0.4,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    scenario.BranchIDs("A"),
					wantDisliked: scenario.BranchIDs("B", "C"),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},
/*		{
			name: "metastable pair-wise conflict set",
			test: func() test {
				scenario := Scenario{
					"A": {
						BranchID:       BranchID{2},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}, ConflictID{1}),
						ApprovalWeight: 0.33,
					},
					"B": {
						BranchID:       BranchID{3},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{0}),
						ApprovalWeight: 0.33,
					},
					"C": {
						BranchID:       BranchID{4},
						ParentBranches: NewBranchIDs(MasterBranchID),
						Conflicting:    NewConflictIDs(ConflictID{1}),
						ApprovalWeight: 0.33,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    scenario.BranchIDs("A"),
					wantDisliked: scenario.BranchIDs("B", "C"),
					args:         scenario.BranchIDs(),
				}
			}(),
			wantErr: false,
		},*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			for name, m := range tt.test.Scenario {
				createTestBranch(t, branchDAG, name, m)
			}
			o := &OnTangleVoting{branchDAG: branchDAG, weightFunc: tt.test.WeightFunc}

			gotLiked, gotDisliked, err := o.Opinion(tt.test.args)
			if (err != nil) != tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.EqualValues(t, tt.test.wantLiked, gotLiked)
			require.EqualValues(t, tt.test.wantDisliked, gotDisliked)
		})
	}
}
