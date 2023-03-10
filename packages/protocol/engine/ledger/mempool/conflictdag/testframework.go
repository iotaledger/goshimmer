package conflictdag

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/options"
)

type TestFramework struct {
	test *testing.T

	Instance *ConflictDAG[utxo.TransactionID, utxo.OutputID]

	conflictIDsByAlias map[string]utxo.TransactionID
	resourceByAlias    map[string]utxo.OutputID

	conflictCreated        int32
	conflictUpdated        int32
	conflictAccepted       int32
	confirmationState      map[utxo.TransactionID]confirmation.State
	conflictRejected       int32
	conflictNotConflicting int32

	optsConflictDAG []options.Option[ConflictDAG[utxo.TransactionID, utxo.OutputID]]
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, conflictDAGInstance *ConflictDAG[utxo.TransactionID, utxo.OutputID]) *TestFramework {
	t := &TestFramework{
		test:               test,
		Instance:           conflictDAGInstance,
		conflictIDsByAlias: make(map[string]utxo.TransactionID),
		resourceByAlias:    make(map[string]utxo.OutputID),
		confirmationState:  make(map[utxo.TransactionID]confirmation.State),
	}
	t.setupEvents()
	return t
}

func NewDefaultTestFramework(t *testing.T, opts ...options.Option[ConflictDAG[utxo.TransactionID, utxo.OutputID]]) *TestFramework {
	return NewTestFramework(t, New(opts...))
}

func (t *TestFramework) setupEvents() {
	t.Instance.Events.ConflictCreated.Hook(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("CREATED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictCreated), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	})
	t.Instance.Events.ConflictUpdated.Hook(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("UPDATED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictUpdated), 1)
	})
	t.Instance.Events.ConflictAccepted.Hook(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("ACCEPTED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictAccepted), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	})
	t.Instance.Events.ConflictRejected.Hook(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("REJECTED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictRejected), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	})
	t.Instance.Events.ConflictNotConflicting.Hook(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("NOT CONFLICTING: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictNotConflicting), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	})
}

func (t *TestFramework) RegisterConflictIDAlias(alias string, conflictID utxo.TransactionID) {
	conflictID.RegisterAlias(alias)
	t.conflictIDsByAlias[alias] = conflictID
}

func (t *TestFramework) RegisterConflictSetIDAlias(alias string, conflictSetID utxo.OutputID) {
	conflictSetID.RegisterAlias(alias)
	t.resourceByAlias[alias] = conflictSetID
}

func (t *TestFramework) CreateConflict(conflictAlias string, parentConflictIDs utxo.TransactionIDs, conflictSetAliases ...string) {
	t.RegisterConflictIDAlias(conflictAlias, t.randomConflictID())
	for _, conflictSetAlias := range conflictSetAliases {
		if _, exists := t.resourceByAlias[conflictSetAlias]; !exists {
			t.RegisterConflictSetIDAlias(conflictSetAlias, t.randomResourceID())
		}
	}

	t.Instance.CreateConflict(t.ConflictID(conflictAlias), parentConflictIDs, t.ConflictSetIDs(conflictSetAliases...), confirmation.Pending)
}

func (t *TestFramework) UpdateConflictingResources(conflictAlias string, conflictingResourcesAliases ...string) {
	t.Instance.UpdateConflictingResources(t.ConflictID(conflictAlias), t.ConflictSetIDs(conflictingResourcesAliases...))
}

func (t *TestFramework) UpdateConflictParents(conflictAlias string, addedConflictAlias string, removedConflictAliases ...string) {
	t.Instance.UpdateConflictParents(t.ConflictID(conflictAlias), t.ConflictIDs(removedConflictAliases...), t.ConflictID(addedConflictAlias))
}

func (t *TestFramework) UnconfirmedConflicts(conflictAliases ...string) *advancedset.AdvancedSet[utxo.TransactionID] {
	return t.Instance.UnconfirmedConflicts(t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework) SetConflictAccepted(conflictAlias string) {
	t.Instance.SetConflictAccepted(t.ConflictID(conflictAlias))
}

func (t *TestFramework) ConfirmationState(conflictAliases ...string) confirmation.State {
	return t.Instance.ConfirmationState(t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework) DetermineVotes(conflictAliases ...string) (addedConflicts, revokedConflicts *advancedset.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	return t.Instance.DetermineVotes(t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework) ConflictID(alias string) (conflictID utxo.TransactionID) {
	conflictID, ok := t.conflictIDsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("ConflictID alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ConflictIDs(aliases ...string) (conflictIDs utxo.TransactionIDs) {
	conflictIDs = utxo.NewTransactionIDs()
	for _, alias := range aliases {
		conflictIDs.Add(t.ConflictID(alias))
	}

	return
}

func (t *TestFramework) ConflictSetID(alias string) (conflictSetID utxo.OutputID) {
	conflictSetID, ok := t.resourceByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("ConflictSetID alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ConflictSetIDs(aliases ...string) (conflictSetIDs utxo.OutputIDs) {
	conflictSetIDs = utxo.NewOutputIDs()
	for _, alias := range aliases {
		conflictSetIDs.Add(t.ConflictSetID(alias))
	}

	return
}

func (t *TestFramework) randomConflictID() (randomConflictID utxo.TransactionID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func (t *TestFramework) randomResourceID() (randomConflictID utxo.OutputID) {
	if err := randomConflictID.FromRandomness(); err != nil {
		panic(err)
	}

	return randomConflictID
}

func (t *TestFramework) assertConflictSets(expectedConflictSets map[string][]string) {
	for conflictSetAlias, conflictAliases := range expectedConflictSets {
		conflictSet, exists := t.Instance.ConflictSet(t.ConflictSetID(conflictSetAlias))
		require.Truef(t.test, exists, "ConflictSet %s not found", conflictSetAlias)

		expectedConflictIDs := t.ConflictIDs(conflictAliases...).Slice()
		actualConflictIDs := lo.Map(conflictSet.Conflicts().Slice(), func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) utxo.TransactionID {
			return conflict.ID()
		})

		require.ElementsMatchf(t.test, expectedConflictIDs, actualConflictIDs, "Expected ConflictSet %s to have conflicts %v but got %v", conflictSetAlias, expectedConflictIDs, actualConflictIDs)
	}
}

func (t *TestFramework) assertConflictsParents(expectedParents map[string][]string) {
	for conflictAlias, parentConflictAliases := range expectedParents {
		conflict, exists := t.Instance.Conflict(t.ConflictID(conflictAlias))
		require.Truef(t.test, exists, "Conflict %s not found", conflictAlias)

		expectedParentConflictIDs := t.ConflictIDs(parentConflictAliases...).Slice()
		require.ElementsMatchf(t.test, expectedParentConflictIDs, conflict.Parents().Slice(), "Expected Conflict %s to have parents %v but got %v", conflictAlias, expectedParentConflictIDs, conflict.Parents().Slice())
	}
}

func (t *TestFramework) assertConflictsChildren(expectedChildren map[string][]string) {
	for conflictAlias, childConflictAliases := range expectedChildren {
		conflict, exists := t.Instance.Conflict(t.ConflictID(conflictAlias))
		require.Truef(t.test, exists, "Conflict %s not found", conflictAlias)

		expectedChildConflictIDs := t.ConflictIDs(childConflictAliases...).Slice()
		actualChildConflictIDs := lo.Map(conflict.Children().Slice(), func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) utxo.TransactionID {
			return conflict.ID()
		})
		require.ElementsMatchf(t.test, expectedChildConflictIDs, actualChildConflictIDs, "Expected Conflict %s to have children %v but got %v", conflictAlias, expectedChildConflictIDs, actualChildConflictIDs)
	}
}

func (t *TestFramework) assertConflictsConflictSets(expectedConflictSets map[string][]string) {
	for conflictAlias, conflictSetAliases := range expectedConflictSets {
		conflict, exists := t.Instance.Conflict(t.ConflictID(conflictAlias))
		require.Truef(t.test, exists, "Conflict %s not found", conflictAlias)

		expectedConflictSetIDs := t.ConflictSetIDs(conflictSetAliases...).Slice()
		actualConflictSetIDs := lo.Map(conflict.ConflictSets().Slice(), func(conflict *ConflictSet[utxo.TransactionID, utxo.OutputID]) utxo.OutputID {
			return conflict.ID()
		})
		require.ElementsMatchf(t.test, expectedConflictSetIDs, actualConflictSetIDs, "Expected Conflict %s to have conflict sets %v but got %v", conflictAlias, expectedConflictSetIDs, actualConflictSetIDs)
	}
}

// AssertConflictParentsAndChildren asserts the structure of the conflict DAG as specified in expectedParents.
// "conflict3": {"conflict1","conflict2"} asserts that "conflict3" should have "conflict1" and "conflict2" as parents.
// It also verifies the reverse mapping, that there is a child reference from "conflict1"->"conflict3" and "conflict2"->"conflict3".
func (t *TestFramework) AssertConflictParentsAndChildren(expectedParents map[string][]string) {
	t.assertConflictsParents(expectedParents)

	expectedChildren := make(map[string][]string)
	for conflictAlias, expectedParentAliases := range expectedParents {
		for _, parentAlias := range expectedParentAliases {
			if _, exists := expectedChildren[parentAlias]; !exists {
				expectedChildren[parentAlias] = make([]string, 0)
			}
			expectedChildren[parentAlias] = append(expectedChildren[parentAlias], conflictAlias)
		}
	}

	t.assertConflictsChildren(expectedChildren)
}

// AssertConflictSetsAndConflicts asserts conflict membership from ConflictSetID -> Conflict but also the reverse mapping Conflict -> ConflictSetID.
// expectedConflictAliases should be specified as
// "conflictSetID1": {"conflict1", "conflict2"}.
func (t *TestFramework) AssertConflictSetsAndConflicts(expectedConflictSetToConflictsAliases map[string][]string) {
	t.assertConflictSets(expectedConflictSetToConflictsAliases)

	// transform to conflict -> expected conflictSetIDs.
	expectedConflictToConflictSetsAliases := make(map[string][]string)
	for resourceAlias, expectedConflictMembersAliases := range expectedConflictSetToConflictsAliases {
		for _, conflictAlias := range expectedConflictMembersAliases {
			if _, exists := expectedConflictToConflictSetsAliases[conflictAlias]; !exists {
				expectedConflictToConflictSetsAliases[conflictAlias] = make([]string, 0)
			}
			expectedConflictToConflictSetsAliases[conflictAlias] = append(expectedConflictToConflictSetsAliases[conflictAlias], resourceAlias)
		}
	}

	t.assertConflictsConflictSets(expectedConflictToConflictSetsAliases)
}

func (t *TestFramework) AssertConfirmationState(expectedConfirmationState map[string]confirmation.State) {
	for conflictAlias, expectedState := range expectedConfirmationState {
		conflictConfirmationState, exists := t.confirmationState[t.ConflictID(conflictAlias)]
		require.Truef(t.test, exists, "Conflict %s not found", conflictAlias)

		require.Equal(t.test, expectedState, conflictConfirmationState, "Expected Conflict %s to have confirmation state %v but got %v", conflictAlias, expectedState, conflictConfirmationState)
	}
}
