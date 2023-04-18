package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
)

type TestFramework[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]] struct {
	Instance            conflictdag.ConflictDAG[ConflictID, ResourceID, VotePower]
	ConflictIDFromAlias func(string) ConflictID
	ResourceIDFromAlias func(string) ResourceID

	test *testing.T
}

func NewTestFramework[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](test *testing.T, instance conflictdag.ConflictDAG[ConflictID, ResourceID, VotePower], conflictIDFromAlias func(string) ConflictID, resourceIDFromAlias func(string) ResourceID) *TestFramework[ConflictID, ResourceID, VotePower] {
	return &TestFramework[ConflictID, ResourceID, VotePower]{
		Instance:            instance,
		ConflictIDFromAlias: conflictIDFromAlias,
		ResourceIDFromAlias: resourceIDFromAlias,
		test:                test,
	}
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) CreateConflict(alias string, parentIDs []string, resourceAliases []string, initialAcceptanceState ...acceptance.State) error {
	return t.Instance.CreateConflict(t.ConflictIDFromAlias(alias), t.ConflictIDs(parentIDs...), t.ConflictSetIDs(resourceAliases...), lo.First(initialAcceptanceState))
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) ConflictIDs(aliases ...string) *advancedset.AdvancedSet[ConflictID] {
	conflictIDs := advancedset.New[ConflictID]()
	for _, alias := range aliases {
		conflictIDs.Add(t.ConflictIDFromAlias(alias))
	}

	return conflictIDs
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) ConflictSetIDs(aliases ...string) *advancedset.AdvancedSet[ResourceID] {
	conflictSetIDs := advancedset.New[ResourceID]()
	for _, alias := range aliases {
		conflictSetIDs.Add(t.ResourceIDFromAlias(alias))
	}

	return conflictSetIDs
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) UpdateConflictParents(conflictAlias string, addedParentID string, removedParentIDs ...string) error {
	return t.Instance.UpdateConflictParents(t.ConflictIDFromAlias(conflictAlias), t.ConflictIDFromAlias(addedParentID), t.ConflictIDs(removedParentIDs...))
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) JoinConflictSets(conflictAlias string, resourceAliases ...string) error {
	return t.Instance.JoinConflictSets(t.ConflictIDFromAlias(conflictAlias), t.ConflictSetIDs(resourceAliases...))
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) LikedInstead(conflictAliases ...string) *advancedset.AdvancedSet[ConflictID] {
	var result *advancedset.AdvancedSet[ConflictID]
	_ = t.Instance.ReadConsistent(func(conflictDAG conflictdag.ReadLockedConflictDAG[ConflictID, ResourceID, VotePower]) error {
		result = conflictDAG.LikedInstead(t.ConflictIDs(conflictAliases...))

		return nil
	})

	return result
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) CastVotes(vote *vote.Vote[VotePower], conflictAliases ...string) error {
	return t.Instance.CastVotes(vote, t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) AssertChildren(conflictAlias string, childAliases ...string) {
	childIDs, exists := t.Instance.ConflictChildren(t.ConflictIDFromAlias(conflictAlias))
	require.True(t.test, exists, "Conflict %s does not exist", conflictAlias)

	require.Equal(t.test, len(childAliases), childIDs.Size(), "Conflict %s has wrong number of children", conflictAlias)
	for _, childAlias := range childAliases {
		require.True(t.test, childIDs.Has(t.ConflictIDFromAlias(childAlias)), "Conflict %s does not have child %s", conflictAlias, childAlias)
	}
}

func (t *TestFramework[ConflictID, ResourceID, VotePower]) AssertParents(conflictAlias string, parentAliases ...string) {
	parents, exists := t.Instance.ConflictParents(t.ConflictIDFromAlias(conflictAlias))
	require.True(t.test, exists, "Conflict %s does not exist", conflictAlias)

	require.Equal(t.test, len(parentAliases), parents.Size(), "Conflict %s has wrong number of parents", conflictAlias)
	for _, parentAlias := range parentAliases {
		require.True(t.test, parents.Has(t.ConflictIDFromAlias(parentAlias)), "Conflict %s does not have parent %s", conflictAlias, parentAlias)
	}
}
