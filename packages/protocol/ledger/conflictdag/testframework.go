package conflictdag

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type TestFramework struct {
	ConflictDAG   *ConflictDAG[utxo.TransactionID, utxo.OutputID]
	evictionState *eviction.State

	test *testing.T

	conflictIDsByAlias map[string]utxo.TransactionID
	resourceByAlias    map[string]utxo.OutputID

	conflictCreated   int32
	conflictUpdated   int32
	conflictAccepted  int32
	confirmationState map[utxo.TransactionID]confirmation.State
	conflictRejected  int32

	optsConflictDAG []options.Option[ConflictDAG[utxo.TransactionID, utxo.OutputID]]
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newFramework *TestFramework) {
	return options.Apply(&TestFramework{
		conflictIDsByAlias: make(map[string]utxo.TransactionID),
		resourceByAlias:    make(map[string]utxo.OutputID),
		confirmationState:  make(map[utxo.TransactionID]confirmation.State),
		test:               test,
	}, opts, func(t *TestFramework) {
		if t.ConflictDAG == nil {
			storageInstance := storage.New(test.TempDir(), 1)
			test.Cleanup(func() {
				storageInstance.Shutdown()
			})

			if t.evictionState == nil {
				t.evictionState = eviction.NewState(storageInstance)
			}

			t.ConflictDAG = New(t.optsConflictDAG...)
		}
	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) setupEvents() {
	t.ConflictDAG.Events.ConflictCreated.Hook(event.NewClosure(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("CREATED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictCreated), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	}))

	t.ConflictDAG.Events.ConflictUpdated.Hook(event.NewClosure(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("UPDATED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictUpdated), 1)
	}))

	t.ConflictDAG.Events.ConflictAccepted.Hook(event.NewClosure(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("ACCEPTED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictAccepted), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	}))

	t.ConflictDAG.Events.ConflictRejected.Hook(event.NewClosure(func(conflict *Conflict[utxo.TransactionID, utxo.OutputID]) {
		if debug.GetEnabled() {
			t.test.Logf("REJECTED: %s", conflict.ID())
		}
		atomic.AddInt32(&(t.conflictRejected), 1)
		t.confirmationState[conflict.ID()] = conflict.ConfirmationState()
	}))
}

func (t *TestFramework) RegisterConflictIDAlias(alias string, conflictID utxo.TransactionID) {
	t.conflictIDsByAlias[alias] = conflictID
}

func (t *TestFramework) RegisterConflictSetIDAlias(alias string, conflictSetID utxo.OutputID) {
	t.resourceByAlias[alias] = conflictSetID
}

func (t *TestFramework) CreateConflict(conflictAlias string, parentConflictIDs utxo.TransactionIDs, conflictSetAliases ...string) {
	for _, conflictSetAlias := range conflictSetAliases {
		if _, exists := t.resourceByAlias[conflictSetAlias]; !exists {
			t.resourceByAlias[conflictSetAlias] = t.randomResourceID()
			t.resourceByAlias[conflictSetAlias].RegisterAlias(conflictSetAlias)
		}
	}

	t.conflictIDsByAlias[conflictAlias] = t.randomConflictID()
	t.conflictIDsByAlias[conflictAlias].RegisterAlias(conflictAlias)

	t.ConflictDAG.CreateConflict(t.ConflictID(conflictAlias), parentConflictIDs, t.ConflictSetIDs(conflictSetAliases...))
}

func (t *TestFramework) UpdateConflictingResources(conflictAlias string, conflictingResourcesAliases ...string) {
	t.ConflictDAG.UpdateConflictingResources(t.ConflictID(conflictAlias), t.ConflictSetIDs(conflictingResourcesAliases...))
}

func (t *TestFramework) UpdateConflictParents(conflictAlias string, addedConflictAlias string, removedConflictAliases ...string) {
	t.ConflictDAG.UpdateConflictParents(t.ConflictID(conflictAlias), t.ConflictIDs(removedConflictAliases...), t.ConflictID(addedConflictAlias))
}

func (t *TestFramework) UnconfirmedConflicts(conflictAliases ...string) *set.AdvancedSet[utxo.TransactionID] {
	return t.ConflictDAG.UnconfirmedConflicts(t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework) SetConflictAccepted(conflictAlias string) {
	t.ConflictDAG.SetConflictAccepted(t.ConflictID(conflictAlias))
}

func (t *TestFramework) ConfirmationState(conflictAliases ...string) confirmation.State {
	return t.ConflictDAG.ConfirmationState(t.ConflictIDs(conflictAliases...))
}

func (t *TestFramework) DetermineVotes(conflictAliases ...string) (addedConflicts, revokedConflicts *set.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	return t.ConflictDAG.DetermineVotes(t.ConflictIDs(conflictAliases...))
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
		conflictSet, exists := t.ConflictDAG.ConflictSet(t.ConflictSetID(conflictSetAlias))
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
		conflict, exists := t.ConflictDAG.Conflict(t.ConflictID(conflictAlias))
		require.Truef(t.test, exists, "Conflict %s not found", conflictAlias)

		expectedParentConflictIDs := t.ConflictIDs(parentConflictAliases...).Slice()
		require.ElementsMatchf(t.test, expectedParentConflictIDs, conflict.Parents().Slice(), "Expected Conflict %s to have parents %v but got %v", conflictAlias, expectedParentConflictIDs, conflict.Parents().Slice())
	}
}

func (t *TestFramework) assertConflictsChildren(expectedChildren map[string][]string) {
	for conflictAlias, childConflictAliases := range expectedChildren {
		conflict, exists := t.ConflictDAG.Conflict(t.ConflictID(conflictAlias))
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
		conflict, exists := t.ConflictDAG.Conflict(t.ConflictID(conflictAlias))
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

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithEvictionState returns an option that sets the eviction state of the TestFramework.
func WithEvictionState(evictionState *eviction.State) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.evictionState = evictionState
	}
}

// WithConflictDAGOptions returns an option that sets the ConflictDAGOptions of the TestFramework.
func WithConflictDAGOptions(opts ...options.Option[ConflictDAG[utxo.TransactionID, utxo.OutputID]]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsConflictDAG = opts
	}
}

// WithConflictDAG returns an option that allows you to provide a BlockDAG instance to the TestFramework.
func WithConflictDAG(conflictDAG *ConflictDAG[utxo.TransactionID, utxo.OutputID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.ConflictDAG = conflictDAG
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
