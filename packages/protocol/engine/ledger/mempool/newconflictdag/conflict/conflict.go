package conflict

import (
	"bytes"
	"sync"

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

	// children is the set of children of the Conflict.
	children *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	// conflictSets is the set of conflict sets that contain the Conflict.
	conflictSets map[ResourceID]*Set[ConflictID, ResourceID]

	// weight is the weight of the Conflict.
	weight *weight.Weight

	// preferredInstead is the preferred instead value of the Conflict.
	preferredInstead *Conflict[ConflictID, ResourceID]

	// likedInstead is the set of liked instead Conflicts.
	likedInstead *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	// likedInsteadSources is a mapping of liked instead Conflicts to the set of parents that inherited them.
	likedInsteadSources *shrinkingmap.ShrinkingMap[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]]

	// conflictingConflicts is the set of conflicts that directly conflict with the Conflict.
	conflictingConflicts *SortedSet[ConflictID, ResourceID]

	// parentsPreferredInstead is a set of preferred instead Conflicts from the parent Conflicts.
	parentsPreferredInstead map[ConflictID]ConflictID

	// RWMutex is used to synchronize access to the Conflict.
	mutex sync.RWMutex
}

func (c *Conflict[ConflictID, ResourceID]) ParentsPreferredInstead() map[ConflictID]ConflictID {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// copy the parentsPreferredInstead map to a new empty one and return the result
	return lo.MergeMaps(make(map[ConflictID]ConflictID), c.parentsPreferredInstead)
}

// New creates a new Conflict.
func New[ConflictID, ResourceID IDType](id ConflictID, parents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]], conflictSets map[ResourceID]*Set[ConflictID, ResourceID], initialWeight *weight.Weight, pendingTasksCounter *syncutils.Counter) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		id:                      id,
		parents:                 parents,
		children:                advancedset.New[*Conflict[ConflictID, ResourceID]](),
		conflictSets:            conflictSets,
		weight:                  initialWeight,
	}
	c.preferredInstead = c
	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c, pendingTasksCounter)

	if parents != nil {
		_ = parents.ForEach(func(parent *Conflict[ConflictID, ResourceID]) (err error) {
			parent.LikedInsteadAdded.Hook(func(likedInstead *Conflict[ConflictID, ResourceID]) {
				c.onParentAddedLikedInstead(parent, likedInstead)
			})

			return nil
		})
	}

	// add existing conflicts first, so we can correctly determine the preferred instead flag
	for _, conflictSet := range conflictSets {
		_ = conflictSet.Members().ForEach(func(element *Conflict[ConflictID, ResourceID]) (err error) {
			c.conflictingConflicts.Add(element)

			return nil
		})
	}

	// add ourselves to the other conflict sets once we are fully initialized
	for _, conflictSet := range conflictSets {
		conflictSet.Add(c)
	}

	// add ourselves to the other conflict sets once we are fully initialized
	for _, conflictSet := range conflictSets {
		conflictSet.Add(c)
	}

	return c
}

// ID returns the identifier of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) ID() ConflictID {
	return c.id
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

// SetPreferredInstead sets the preferred instead value of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) SetPreferredInstead(preferredInstead *Conflict[ConflictID, ResourceID]) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.preferredInstead == preferredInstead {
		return false
	}

	c.preferredInstead = preferredInstead

	c.PreferredInsteadUpdated.Trigger(preferredInstead)

	_ = c.children.ForEach(func(child *Conflict[ConflictID, ResourceID]) (err error) {
		child.SetParentPreferredInstead(c.ID(), preferredInstead.ID())
		return nil
	})

	return true
}

func (c *Conflict[ConflictID, ResourceID]) SetParentPreferredInstead(parent, preferredInstead ConflictID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if parent != preferredInstead {
		c.parentsPreferredInstead[parent] = preferredInstead
	} else {
		delete(c.parentsPreferredInstead, parent)
	}

	_ = c.children.ForEach(func(child *Conflict[ConflictID, ResourceID]) (err error) {
		child.SetParentPreferredInstead(parent, preferredInstead)
		return nil
	})
}

// IsPreferred returns true if the Conflict is preferred instead of its conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID]) IsPreferred() bool {
	return c.PreferredInstead() == c
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

// String returns a human-readable representation of the Conflict.
func (c *Conflict[ConflictID, ResourceID]) String() string {
	return stringify.Struct("Conflict",
		stringify.NewStructField("id", c.id),
		stringify.NewStructField("weight", c.weight),
	)
}

func (c *Conflict[ConflictID, ResourceID]) LikedInstead() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.likedInstead.Clone()
}

func (c *Conflict[ConflictID, ResourceID]) IsLiked() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.likedInstead.Size() == 0 && c.preferredInstead == c
}

func (c *Conflict[ConflictID, ResourceID]) onParentAddedLikedInstead(parent *Conflict[ConflictID, ResourceID], likedConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	likedInsteadSources := lo.Return1(c.likedInsteadSources.GetOrCreate(likedConflict.ID(), func() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
		return advancedset.New[*Conflict[ConflictID, ResourceID]]()
	}))

	if !likedInsteadSources.Add(parent) || !c.likedInstead.Add(likedConflict) {
		return
	}

	c.LikedInsteadAdded.Trigger(likedConflict)
}

func (c *Conflict[ConflictID, ResourceID]) onParentRemovedLikedInstead(parent *Conflict[ConflictID, ResourceID], likedConflict *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sources := lo.Return1(c.likedInsteadSources.Get(likedConflict.ID()))
	if sources == nil || !sources.Delete(parent) || !sources.IsEmpty() {
		return
	}

	if !c.likedInsteadSources.Delete(likedConflict.ID()) || !c.likedInstead.Delete(likedConflict) {
		return
	}

	c.LikedInsteadRemoved.Trigger(likedConflict)
}
