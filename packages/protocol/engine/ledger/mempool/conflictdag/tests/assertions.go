package tests

import (
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
)

// Assertions provides a set of assertions for the ConflictDAG.
type Assertions[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]] struct {
	f *Framework[ConflictID, ResourceID, VotePower]
}

// Children asserts that the given conflict has the given children.
func (a *Assertions[ConflictID, ResourceID, VotePower]) Children(conflictAlias string, childAliases ...string) {
	childIDs, exists := a.f.Instance.ConflictChildren(a.f.ConflictID(conflictAlias))
	require.True(a.f.test, exists, "Conflict %s does not exist", conflictAlias)

	require.Equal(a.f.test, len(childAliases), childIDs.Size(), "Conflict %s has wrong number of children", conflictAlias)
	for _, childAlias := range childAliases {
		require.True(a.f.test, childIDs.Has(a.f.ConflictID(childAlias)), "Conflict %s does not have child %s", conflictAlias, childAlias)
	}
}

// Parents asserts that the given conflict has the given parents.
func (a *Assertions[ConflictID, ResourceID, VotePower]) Parents(conflictAlias string, parentAliases ...string) {
	parents, exists := a.f.Instance.ConflictParents(a.f.ConflictID(conflictAlias))
	require.True(a.f.test, exists, "Conflict %s does not exist", conflictAlias)

	require.Equal(a.f.test, len(parentAliases), parents.Size(), "Conflict %s has wrong number of parents", conflictAlias)
	for _, parentAlias := range parentAliases {
		require.True(a.f.test, parents.Has(a.f.ConflictID(parentAlias)), "Conflict %s does not have parent %s", conflictAlias, parentAlias)
	}
}

// LikedInstead asserts that the given conflicts return the given LikedInstead conflicts.
func (a *Assertions[ConflictID, ResourceID, VotePower]) LikedInstead(conflictAliases []string, likedInsteadAliases ...string) {
	likedInsteadConflicts := a.f.LikedInstead(conflictAliases...)

	require.Equal(a.f.test, len(likedInsteadAliases), likedInsteadConflicts.Size(), "LikedInstead returns wrong number of conflicts %d instead of %d", likedInsteadConflicts.Size(), len(likedInsteadAliases))
}

// ConflictSetMembers asserts that the given resource has the given conflict set members.
func (a *Assertions[ConflictID, ResourceID, VotePower]) ConflictSetMembers(resourceAlias string, conflictAliases ...string) {
	conflictSetMembers, exists := a.f.Instance.ConflictSetMembers(a.f.ResourceID(resourceAlias))
	require.True(a.f.test, exists, "Resource %s does not exist", resourceAlias)

	require.Equal(a.f.test, len(conflictAliases), conflictSetMembers.Size(), "Resource %s has wrong number of parents", resourceAlias)
	for _, conflictAlias := range conflictAliases {
		require.True(a.f.test, conflictSetMembers.Has(a.f.ConflictID(conflictAlias)), "Resource %s does not have parent %s", resourceAlias, conflictAlias)
	}
}

// ConflictSets asserts that the given conflict has the given conflict sets.
func (a *Assertions[ConflictID, ResourceID, VotePower]) ConflictSets(conflictAlias string, resourceAliases ...string) {
	conflictSets, exists := a.f.Instance.ConflictSets(a.f.ConflictID(conflictAlias))
	require.True(a.f.test, exists, "Conflict %s does not exist", conflictAlias)

	require.Equal(a.f.test, len(resourceAliases), conflictSets.Size(), "Conflict %s has wrong number of conflict sets", conflictAlias)
	for _, resourceAlias := range resourceAliases {
		require.True(a.f.test, conflictSets.Has(a.f.ResourceID(resourceAlias)), "Conflict %s does not have conflict set %s", conflictAlias, resourceAlias)
	}
}

// Pending asserts that the given conflicts are pending.
func (a *Assertions[ConflictID, ResourceID, VotePower]) Pending(aliases ...string) {
	for _, alias := range aliases {
		require.True(a.f.test, a.f.Instance.AcceptanceState(a.f.ConflictIDs(alias)).IsPending(), "Conflict %s is not pending", alias)
	}
}

// Accepted asserts that the given conflicts are accepted.
func (a *Assertions[ConflictID, ResourceID, VotePower]) Accepted(aliases ...string) {
	for _, alias := range aliases {
		require.True(a.f.test, a.f.Instance.AcceptanceState(a.f.ConflictIDs(alias)).IsAccepted(), "Conflict %s is not accepted", alias)
	}
}

// Rejected asserts that the given conflicts are rejected.
func (a *Assertions[ConflictID, ResourceID, VotePower]) Rejected(aliases ...string) {
	for _, alias := range aliases {
		require.True(a.f.test, a.f.Instance.AcceptanceState(a.f.ConflictIDs(alias)).IsRejected(), "Conflict %s is not rejected", alias)
	}
}

// ValidatorWeight asserts that the given conflict has the given validator weight.
func (a *Assertions[ConflictID, ResourceID, VotePower]) ValidatorWeight(conflictAlias string, weight int64) {
	require.Equal(a.f.test, weight, a.f.Instance.ConflictWeight(a.f.ConflictID(conflictAlias)), "ValidatorWeight is %s instead of % for conflict %s", a.f.Instance.ConflictWeight(a.f.ConflictID(conflictAlias)), weight, conflictAlias)
}
