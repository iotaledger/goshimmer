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
	Accepted *event.Event

	Rejected *event.Event

	// PreferredInsteadUpdated is triggered when the preferred instead value of the Conflict is updated.
	PreferredInsteadUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	// LikedInsteadAdded is triggered when a liked instead reference is added to the Conflict.
	LikedInsteadAdded *event.Event1[*Conflict[ConflictID, ResourceID]]

	// LikedInsteadRemoved is triggered when a liked instead reference is removed from the Conflict.
	LikedInsteadRemoved *event.Event1[*Conflict[ConflictID, ResourceID]]

	// id is the identifier of the Conflict.
	id ConflictID

	// parents is the set of parents of the Conflict.
	parents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

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
		Accepted:                event.New(),
		Rejected:                event.New(),
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		LikedInsteadAdded:       event.New1[*Conflict[ConflictID, ResourceID]](),
		LikedInsteadRemoved:     event.New1[*Conflict[ConflictID, ResourceID]](),

		id:                  id,
		parents:             advancedset.New[*Conflict[ConflictID, ResourceID]](),
		childUnhooks:        shrinkingmap.New[ConflictID, func()](),
		conflictSets:        advancedset.New[*Set[ConflictID, ResourceID]](),
		weight:              initialWeight,
		likedInstead:        advancedset.New[*Conflict[ConflictID, ResourceID]](),
		likedInsteadSources: shrinkingmap.New[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]](),
	}

	c.preferredInstead = c
	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c, pendingTasksCounter)

	c.RegisterWithConflictSets(conflictSets...)
	c.RegisterWithParents(parents...)

	return c
}

// ID returns the identifier of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) ID() ConflictID {
	return c.id
}

// Parents returns the set of parents of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) Parents() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.parents.Clone()
}

// RegisterWithConflictSets registers the Conflict with the given ConflictSets.
func (c *Conflict[ConflictID, ResourceID]) RegisterWithConflictSets(conflictSets ...*Set[ConflictID, ResourceID]) map[ResourceID]*Set[ConflictID, ResourceID] {
	joinedConflictSets := make(map[ResourceID]*Set[ConflictID, ResourceID], 0)
	for _, conflictSet := range conflictSets {
		if c.conflictSets.Add(conflictSet) {
			// add existing first, so we can determine our own status in respect to the existing conflicts
			_ = conflictSet.Members().ForEach(func(conflict *Conflict[ConflictID, ResourceID]) (err error) {
				c.addConflictingConflict(conflict)

				return nil
			})

			// add ourselves to the other conflict sets once we are fully initialized
			conflictSet.Add(c)

			// add the conflict set to the joined conflict sets
			joinedConflictSets[conflictSet.ID()] = conflictSet
		}
	}

	return joinedConflictSets
}

func (c *Conflict[ConflictID, ResourceID]) RegisterWithParents(parents ...*Conflict[ConflictID, ResourceID]) (registered bool) {
	for _, parent := range parents {
		registered = c.UpdateParents(parent) || registered
	}

	return registered
}

func (c *Conflict[ConflictID, ResourceID]) UnregisterChild(conflictID ConflictID) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if unhookFunc, exists := c.childUnhooks.Get(conflictID); exists {
		unhookFunc()

		c.childUnhooks.Delete(conflictID)
	}
}

func (c *Conflict[ConflictID, ResourceID]) RegisterChild(newChild *Conflict[ConflictID, ResourceID]) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.childUnhooks.Set(newChild.ID(), lo.Batch(
		c.Rejected.Hook(func() {
			if newChild.SetRejected() {
				newChild.Rejected.Trigger()
			}
		}).Unhook,

		c.LikedInsteadRemoved.Hook(func(conflict *Conflict[ConflictID, ResourceID]) {
			newChild.onParentRemovedLikedInstead(c, conflict)
		}).Unhook,

		c.LikedInsteadAdded.Hook(func(conflict *Conflict[ConflictID, ResourceID]) {
			newChild.onParentAddedLikedInstead(c, conflict)
		}).Unhook,
	))

	for conflicts := c.likedInstead.Iterator(); conflicts.HasNext(); {
		newChild.onParentAddedLikedInstead(c, conflicts.Next())
	}

	return c.isRejected() && newChild.setRejected()
}

func (c *Conflict[ConflictID, ResourceID]) UpdateParents(addedParent *Conflict[ConflictID, ResourceID], removedParents ...*Conflict[ConflictID, ResourceID]) (updated bool) {
	if isRejected := func() bool {
		c.mutex.Lock()
		defer c.mutex.Unlock()

		for _, removedParent := range removedParents {
			if c.parents.Delete(removedParent) {
				removedParent.UnregisterChild(c.id)

				updated = true
			}
		}

		if !c.parents.Add(addedParent) {
			return false
		}

		updated = true

		return addedParent.RegisterChild(c)
	}(); isRejected {
		c.Rejected.Trigger()
	}

	return
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

func (c *Conflict[ConflictID, ResourceID]) IsPending() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.weight.Value().AcceptanceState() == acceptance.Pending
}

func (c *Conflict[ConflictID, ResourceID]) IsAccepted() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isAccepted()
}

func (c *Conflict[ConflictID, ResourceID]) SetAccepted() (updated bool) {
	if updated = c.setAccepted(); updated {
		c.Accepted.Trigger()
	}

	return updated
}

func (c *Conflict[ConflictID, ResourceID]) IsRejected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isRejected()
}

// SetRejected sets the Conflict as rejected.
func (c *Conflict[ConflictID, ResourceID]) SetRejected() (updated bool) {
	c.mutex.Lock()
	updated = c.setRejected()
	c.mutex.Unlock()

	if updated {
		c.Rejected.Trigger()
	}

	return updated
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

	return bytes.Compare(lo.PanicOnErr(c.id.Bytes()), lo.PanicOnErr(other.id.Bytes()))
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
		stringify.NewStructField("id", c.id),
		stringify.NewStructField("weight", c.weight),
	)
}

func (c *Conflict[ConflictID, ResourceID]) isAccepted() bool {
	return c.weight.AcceptanceState() == acceptance.Accepted
}

func (c *Conflict[ConflictID, ResourceID]) setAccepted() (updated bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.isAccepted() {
		return false
	}

	_ = c.parents.ForEach(func(parent *Conflict[ConflictID, ResourceID]) (err error) {
		parent.SetAccepted()
		return nil
	})

	c.weight.SetAcceptanceState(acceptance.Accepted)

	return true
}

func (c *Conflict[ConflictID, ResourceID]) isRejected() bool {
	return c.weight.Value().AcceptanceState() == acceptance.Rejected
}

// setRejected sets the Conflict as rejected (without locking).
func (c *Conflict[ConflictID, ResourceID]) setRejected() (updated bool) {
	if c.isRejected() {
		return false
	}

	c.weight.SetAcceptanceState(acceptance.Rejected)

	return true
}

// IsPreferred returns true if the Conflict is preferred instead of its conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID]) isPreferred() bool {
	return c.preferredInstead == c
}

func (c *Conflict[ConflictID, ResourceID]) addConflictingConflict(conflict *Conflict[ConflictID, ResourceID]) (added bool) {
	if added = c.conflictingConflicts.Add(conflict); added {
		c.HookStopped(conflict.Accepted.Hook(func() { c.SetRejected() }).Unhook)

		if conflict.IsAccepted() {
			c.SetRejected()
		}
	}

	return added
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

// onParentAddedLikedInstead is called when a parent Conflict adds a liked instead Conflict.
func (c *Conflict[ConflictID, ResourceID]) onParentAddedLikedInstead(parent *Conflict[ConflictID, ResourceID], likedConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sources, sourcesExist := c.likedInsteadSources.Get(likedConflict.ID())
	if !sourcesExist {
		sources = advancedset.New[*Conflict[ConflictID, ResourceID]]()
		c.likedInsteadSources.Set(likedConflict.ID(), sources)
	}

	if !sources.Add(parent) || !c.likedInstead.Add(likedConflict) {
		return
	}

	if c.likedInstead.Delete(c.preferredInstead) {
		c.LikedInsteadRemoved.Trigger(c.preferredInstead)
	}

	c.LikedInsteadAdded.Trigger(likedConflict)
}

// onParentRemovedLikedInstead is called when a parent Conflict removes a liked instead Conflict.
func (c *Conflict[ConflictID, ResourceID]) onParentRemovedLikedInstead(parent *Conflict[ConflictID, ResourceID], likedConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sources, sourcesExist := c.likedInsteadSources.Get(likedConflict.ID())
	if !sourcesExist || !sources.Delete(parent) || !sources.IsEmpty() || !c.likedInsteadSources.Delete(likedConflict.ID()) || !c.likedInstead.Delete(likedConflict) {
		return
	}

	c.LikedInsteadRemoved.Trigger(likedConflict)

	if !c.isPreferred() && c.likedInstead.IsEmpty() {
		c.likedInstead.Add(c.preferredInstead)

		c.LikedInsteadAdded.Trigger(c.preferredInstead)
	}
}
