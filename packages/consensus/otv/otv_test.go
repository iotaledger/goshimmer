package otv

import (
	"fmt"
	"sort"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
	. "github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type BranchMeta struct {
	Order          int
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

func TestOnTangleVoting_LikedInstead(t *testing.T) {

	type ExpectedBranchFunc func(branchIDs BranchIDs)

	mustMatch := func(expected BranchIDs) ExpectedBranchFunc {
		return func(actual BranchIDs) {
			require.EqualValues(t, expected, actual)
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
					wantLiked:  mustMatch(scenario.BranchIDs("A")),
					args:       scenario.BranchID("B"),
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
					wantLiked:  mustMatch(NewBranchIDs()),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{2})),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{2})),
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
					wantLiked:  mustMatch(NewBranchIDs()),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{4}, BranchID{3})),
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
					wantLiked:  mustMatch(NewBranchIDs()),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{3}, BranchID{5})),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{2})),
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{3}, BranchID{5})),
					args:       BranchID{2},
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
					wantLiked:  mustMatch(NewBranchIDs(BranchID{4})),
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
			o := NewOnTangleVoting(tt.test.WeightFunc, branchDAG)

			liked, err := o.LikedInstead(tt.test.args)
			require.NoError(t, err)
			tt.test.wantLiked(liked)
		})
	}
}

func TestOnTangleVoting_Opinion(t *testing.T) {
	type ExpectedBranchesFunc func(branchIDs BranchIDs)
	type ExpectedLikedInsteadFunc func(likedInstead map[BranchID]BranchIDs)
	type ArgsFunc func() (branchIDs BranchIDs)

	mustMatch := func(s *Scenario, aliases ...string) ExpectedBranchesFunc {
		return func(actual BranchIDs) {
			if !assert.EqualValues(t, s.BranchIDs(aliases...), actual) {
				fmt.Println("expected", s.BranchIDs(aliases...))
				fmt.Println("actual", actual)
			}
		}
	}

	mustMatchLikedInstead := func(s *Scenario, likedInsteadAliases map[string][]string) ExpectedLikedInsteadFunc {
		return func(actualLikedInstead map[BranchID]BranchIDs) {
			expectedLikedInstead := map[BranchID]BranchIDs{}

			for b, li := range likedInsteadAliases {
				if len(li) == 0 {
					expectedLikedInstead[s.BranchID(b)] = NewBranchIDs()
					continue
				}
				expectedLikedInstead[s.BranchID(b)] = s.BranchIDs(li...)
			}

			if !assert.EqualValues(t, expectedLikedInstead, actualLikedInstead) {
				fmt.Println("expected", expectedLikedInstead)
				fmt.Println("actual", actualLikedInstead)
			}
		}
	}

	argsFunc := func(s *Scenario, aliases ...string) ArgsFunc {
		return func() (branchIDs BranchIDs) {
			return s.BranchIDs(aliases...)
		}
	}

	type test struct {
		Scenario         Scenario
		WeightFunc       WeightFunc
		args             ArgsFunc
		wantLiked        ExpectedBranchesFunc
		wantDisliked     ExpectedBranchesFunc
		wantLikedInstead ExpectedLikedInsteadFunc
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
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "C"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B", "C"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A"},
						"C": {"A"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "A"),
					wantDisliked: mustMatch(&scenario, "B", "C"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A"},
						"C": {"A"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "C"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "C"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "C"),
					wantDisliked: mustMatch(&scenario, "A", "D"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "C"},
						"D": {"B"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "D"),
					wantDisliked: mustMatch(&scenario, "A", "C"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "D"},
						"C": {"D"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "A", "E"),
					wantDisliked: mustMatch(&scenario, "B", "C", "D"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A", "E"},
						"C": {"A"},
						"D": {"A"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "B", "D"),
					wantDisliked: mustMatch(&scenario, "A", "C", "E"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B", "D"},
						"C": {"D"},
						"E": {"B"},
					}),
					args: argsFunc(&scenario),
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
					wantLiked:    mustMatch(&scenario, "C"),
					wantDisliked: mustMatch(&scenario, "A", "B"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"C"},
						"B": {"C"},
					}),
					args: argsFunc(&scenario),
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
						Order:          1,
						ParentBranches: NewBranchIDs(BranchID{4}, BranchID{6}),
						Conflicting:    NewConflictIDs(),
						ApprovalWeight: 0.35,
						IsAggregated:   true,
					},
				}

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "E"),
					wantDisliked: mustMatch(&scenario, "A", "C", "D", "C+E"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A":   {"B"},
						"C":   {"B"},
						"D":   {"E"},
						"C+E": {"B"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "13",
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "E", "C+E"),
					wantDisliked: mustMatch(&scenario, "B", "D"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A", "C"},
						"D": {"E"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "14",
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "E", "G", "H", "K", "C+E", "C+E+G"),
					wantDisliked: mustMatch(&scenario, "B", "D", "F", "J", "I"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A", "C"},
						"D": {"E"},
						"F": {"G"},
						"J": {"K"},
						"I": {"H"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "15",
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B", "E", "H", "K"),
					wantDisliked: mustMatch(&scenario, "A", "C", "D", "F", "G", "J", "I", "C+E", "C+E+G"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A":     {"B"},
						"C":     {"B"},
						"D":     {"E"},
						"F":     {"B"},
						"G":     {"B"},
						"J":     {"K"},
						"I":     {"H"},
						"C+E":   {"B"},
						"C+E+G": {"B"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "16",
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "B"),
					wantDisliked: mustMatch(&scenario, "A", "C", "F", "G", "H"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"A": {"B"},
						"C": {"B"},
						"F": {"B"},
						"G": {"B"},
						"H": {"B"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "17",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "C", "G"),
					wantDisliked: mustMatch(&scenario, "F", "B", "H"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A", "C"},
						"F": {"G"},
						"H": {"C", "G"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "18",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "H", "L", "N", "J", "J+N"),
					wantDisliked: mustMatch(&scenario, "F", "B", "G", "C", "I", "K", "M", "O"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B": {"A", "H"},
						"C": {"H"},
						"K": {"L"},
						"F": {"H"},
						"G": {"H"},
						"M": {"N"},
						"I": {"J"},
						"O": {"J", "N"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
		{
			name: "19",
			test: func() test {
				scenario := Scenario{
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

				return test{
					Scenario:     scenario,
					WeightFunc:   WeightFuncFromScenario(t, scenario),
					wantLiked:    mustMatch(&scenario, "A", "H", "L", "O"),
					wantDisliked: mustMatch(&scenario, "F", "B", "G", "C", "I", "N", "K", "M", "J", "J+N"),
					wantLikedInstead: mustMatchLikedInstead(&scenario, map[string][]string{
						"B":   {"A", "H"},
						"C":   {"H"},
						"K":   {"L"},
						"F":   {"H"},
						"G":   {"H"},
						"M":   {"O"},
						"N":   {"O"},
						"I":   {"O"},
						"J":   {"O"},
						"J+N": {"O"},
					}),
					args: argsFunc(&scenario),
				}
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
			//defer branchDAG.Shutdown()

			tt.test.Scenario.CreateBranches(t, branchDAG)
			o := NewOnTangleVoting(tt.test.WeightFunc, branchDAG)

			gotLiked, gotDisliked, likedInstead, err := o.Opinion(tt.test.args())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			tt.test.wantLiked(gotLiked)
			tt.test.wantDisliked(gotDisliked)
			tt.test.wantLikedInstead(likedInstead)
		})
	}
}
