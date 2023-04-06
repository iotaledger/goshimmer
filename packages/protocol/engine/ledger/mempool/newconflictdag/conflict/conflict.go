package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

// Conflict is a conflict that is part of a Conflict DAG.
type Conflict[ConflictID, ResourceID IDType] struct {
	AcceptanceStateUpdated *event.Event2[acceptance.State, acceptance.State]

	// PreferredInsteadUpdated is triggered when the preferred instead value of the Conflict is updated.
	PreferredInsteadUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	// LikedInsteadAdded is triggered when a liked instead reference is added to the Conflict.
	LikedInsteadAdded *event.Event1[*Conflict[ConflictID, ResourceID]]

	// LikedInsteadRemoved is triggered when a liked instead reference is removed from the Conflict.
	LikedInsteadRemoved *event.Event1[*Conflict[ConflictID, ResourceID]]

	// ID is the identifier of the Conflict.
	ID ConflictID

	// parents is the set of parents of the Conflict.
	Parents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	// childUnhooks is a mapping of children to their unhook functions.
	childUnhooks *shrinkingmap.ShrinkingMap[ConflictID, func()]

	// conflictSets is the set of conflict sets that contain the Conflict.
	conflictSets *advancedset.AdvancedSet[*Set[ConflictID, ResourceID]]

	// conflictingConflicts is the set of conflicts that directly conflict with the Conflict.
	conflictingConflicts *SortedSet[ConflictID, ResourceID]

	// weight is the weight of the Conflict.
	weight *weight.Weight

	// preferredInstead is the preferred instead value of the Conflict.
	preferredInstead *Conflict[ConflictID, ResourceID]

	// likedInstead is the set of liked instead Conflicts.
	likedInstead *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	// likedInsteadSources is a mapping of liked instead Conflicts to the set of parents that inherited them.
	likedInsteadSources *shrinkingmap.ShrinkingMap[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]]

	// RWMutex is used to synchronize access to the Conflict.
	mutex sync.RWMutex

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// New creates a new Conflict.
func New[ConflictID, ResourceID IDType](id ConflictID, parents []*Conflict[ConflictID, ResourceID], conflictSets []*Set[ConflictID, ResourceID], initialWeight *weight.Weight, pendingTasksCounter *syncutils.Counter) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		AcceptanceStateUpdated:  event.New2[acceptance.State, acceptance.State](),
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		LikedInsteadAdded:       event.New1[*Conflict[ConflictID, ResourceID]](),
		LikedInsteadRemoved:     event.New1[*Conflict[ConflictID, ResourceID]](),

		ID:                  id,
		Parents:             advancedset.New[*Conflict[ConflictID, ResourceID]](),
		childUnhooks:        shrinkingmap.New[ConflictID, func()](),
		conflictSets:        advancedset.New[*Set[ConflictID, ResourceID]](),
		weight:              initialWeight,
		likedInstead:        advancedset.New[*Conflict[ConflictID, ResourceID]](),
		likedInsteadSources: shrinkingmap.New[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]](),
	}

	c.preferredInstead = c
	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c, pendingTasksCounter)

	c.AddConflictSets(conflictSets...)

	for _, parent := range parents {
		c.UpdateParents(parent)
	}

	return c
}

func (c *Conflict[ConflictID, ResourceID]) addConflictingConflict(conflict *Conflict[ConflictID, ResourceID]) (added bool) {
	if added = c.conflictingConflicts.Add(conflict); added {
		c.HookStopped(conflict.AcceptanceStateUpdated.Hook(func(_, newState acceptance.State) {
			if newState.IsAccepted() {
				c.SetAcceptanceState(acceptance.Rejected)
			}
		}).Unhook)

		if conflict.AcceptanceState().IsAccepted() {
			c.SetAcceptanceState(acceptance.Rejected)
		}
	}

	return added
}

// AddConflictSets registers the Conflict with the given ConflictSets.
func (c *Conflict[ConflictID, ResourceID]) AddConflictSets(conflictSets ...*Set[ConflictID, ResourceID]) (addedConflictSets map[ResourceID]*Set[ConflictID, ResourceID]) {
	addedConflictSets = make(map[ResourceID]*Set[ConflictID, ResourceID], 0)
	for _, conflictSet := range conflictSets {
		if c.conflictSets.Add(conflictSet) {
			conflictSet.Add(c)

			addedConflictSets[conflictSet.ID] = conflictSet
		}
	}

	return addedConflictSets
}

func (c *Conflict[ConflictID, ResourceID]) addLikedInsteadReference(source, reference *Conflict[ConflictID, ResourceID]) {
	// retrieve sources for the reference
	sources := lo.Return1(c.likedInsteadSources.GetOrCreate(reference.ID, func() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
		return advancedset.New[*Conflict[ConflictID, ResourceID]]()
	}))

	// abort if the reference did already exist
	if !sources.Add(source) || !c.likedInstead.Add(reference) {
		return
	}

	// remove the "preferred instead reference" if the parent has a "more general liked instead reference"
	if c.likedInstead.Delete(c.preferredInstead) {
		c.LikedInsteadRemoved.Trigger(c.preferredInstead)
	}

	c.LikedInsteadAdded.Trigger(reference)
}

func (c *Conflict[ConflictID, ResourceID]) removeLikedInsteadReference(source, reference *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if sources, sourcesExist := c.likedInsteadSources.Get(reference.ID); !sourcesExist || !sources.Delete(source) || !sources.IsEmpty() || !c.likedInsteadSources.Delete(reference.ID) || !c.likedInstead.Delete(reference) {
		return
	}

	c.LikedInsteadRemoved.Trigger(reference)

	if !c.isPreferred() && c.likedInstead.IsEmpty() {
		c.likedInstead.Add(c.preferredInstead)

		c.LikedInsteadAdded.Trigger(c.preferredInstead)
	}
}

func (c *Conflict[ConflictID, ResourceID]) registerChild(child *Conflict[ConflictID, ResourceID]) acceptance.State {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.childUnhooks.Set(child.ID, lo.Batch(
		c.AcceptanceStateUpdated.Hook(func(_, newState acceptance.State) {
			if newState.IsRejected() {
				child.SetAcceptanceState(newState)
			}
		}).Unhook,

		c.LikedInsteadRemoved.Hook(func(reference *Conflict[ConflictID, ResourceID]) {
			child.removeLikedInsteadReference(c, reference)
		}).Unhook,

		c.LikedInsteadAdded.Hook(func(conflict *Conflict[ConflictID, ResourceID]) {
			child.mutex.Lock()
			defer child.mutex.Unlock()

			child.addLikedInsteadReference(c, conflict)
		}).Unhook,
	))

	for conflicts := c.likedInstead.Iterator(); conflicts.HasNext(); {
		child.addLikedInsteadReference(c, conflicts.Next())
	}

	return c.weight.Value().AcceptanceState()
}

func (c *Conflict[ConflictID, ResourceID]) UpdateParents(addedParent *Conflict[ConflictID, ResourceID], removedParents ...*Conflict[ConflictID, ResourceID]) (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, removedParent := range removedParents {
		if c.Parents.Delete(removedParent) {
			removedParent.unregisterChild(c.ID)
			updated = true
		}
	}

	if !c.Parents.Add(addedParent) {
		return updated
	}

	if addedParent.registerChild(c) == acceptance.Rejected {
		c.SetAcceptanceState(acceptance.Rejected)
	}

	return true
}

// IsLiked returns true if the Conflict is liked instead of other conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID]) IsLiked() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isPreferred() && c.likedInstead.IsEmpty()
}

// IsPreferred returns true if the Conflict is preferred instead of its conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID]) IsPreferred() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isPreferred()
}

func (c *Conflict[ConflictID, ResourceID]) AcceptanceState() acceptance.State {
	return c.weight.Value().AcceptanceState()
}

func (c *Conflict[ConflictID, ResourceID]) SetAcceptanceState(newState acceptance.State) (previousState acceptance.State) {
	if newState == acceptance.Accepted {
		_ = c.Parents.ForEach(func(parent *Conflict[ConflictID, ResourceID]) (err error) {
			parent.SetAcceptanceState(acceptance.Accepted)
			return nil
		})
	}

	if previousState = c.weight.SetAcceptanceState(newState); previousState != newState {
		c.AcceptanceStateUpdated.Trigger(previousState, newState)
	}

	return previousState
}

// Weight returns the weight of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) Weight() *weight.Weight {
	return c.weight
}

// PreferredInstead returns the preferred instead value of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.preferredInstead
}

// LikedInstead returns the set of liked instead Conflicts.
func (c *Conflict[ConflictID, ResourceID]) LikedInstead() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.likedInstead.Clone()
}

// Compare compares the Conflict to the given other Conflict.
func (c *Conflict[ConflictID, ResourceID]) Compare(other *Conflict[ConflictID, ResourceID]) int {
	if c == other {
		return weight.Equal
	}

	if other == nil {
		return weight.Heavier
	}

	if c == nil {
		return weight.Lighter
	}

	if result := c.weight.Compare(other.weight); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(c.ID.Bytes()), lo.PanicOnErr(other.ID.Bytes()))
}

// ForEachConflictingConflict iterates over all conflicting Conflicts of the Conflict and calls the given callback for each of them.
func (c *Conflict[ConflictID, ResourceID]) ForEachConflictingConflict(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for currentMember := c.conflictingConflicts.heaviestMember; currentMember != nil; currentMember = currentMember.lighterMember {
		if currentMember.Conflict != c {
			if err := callback(currentMember.Conflict); err != nil {
				return err
			}
		}
	}

	return nil
}

// String returns a human-readable representation of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) String() string {
	return stringify.Struct("Conflict",
		stringify.NewStructField("id", c.ID),
		stringify.NewStructField("weight", c.weight),
	)
}

// IsPreferred returns true if the Conflict is preferred instead of its conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID]) isPreferred() bool {
	return c.preferredInstead == c
}

func (c *Conflict[ConflictID, ResourceID]) unregisterChild(conflictID ConflictID) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if unhookFunc, exists := c.childUnhooks.Get(conflictID); exists {
		c.childUnhooks.Delete(conflictID)

		unhookFunc()
	}
}

// setPreferredInstead sets the preferred instead value of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) setPreferredInstead(preferredInstead *Conflict[ConflictID, ResourceID]) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.preferredInstead == preferredInstead {
		return false
	}

	previousPreferredInstead := c.preferredInstead
	c.preferredInstead = preferredInstead

	c.PreferredInsteadUpdated.Trigger(preferredInstead)

	if c.likedInstead.Delete(previousPreferredInstead) {
		c.LikedInsteadRemoved.Trigger(previousPreferredInstead)
	}

	if !c.isPreferred() && c.likedInstead.IsEmpty() {
		c.likedInstead.Add(preferredInstead)

		c.LikedInsteadAdded.Trigger(preferredInstead)
	}

	return true
}
