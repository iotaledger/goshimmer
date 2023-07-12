package conflictdagv1

import (
	"bytes"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/core/acceptance"
	"github.com/iotaledger/goshimmer/packages/core/vote"
	"github.com/iotaledger/goshimmer/packages/core/weight"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

// Conflict is a conflict that is part of a Conflict DAG.
type Conflict[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]] struct {
	// ID is the identifier of the Conflict.
	ID ConflictID

	// Parents is the set of parents of the Conflict.
	Parents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	// Children is the set of children of the Conflict.
	Children *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	// ConflictSets is the set of ConflictSets that the Conflict is part of.
	ConflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictID, ResourceID, VotePower]]

	// ConflictingConflicts is the set of conflicts that directly conflict with the Conflict.
	ConflictingConflicts *SortedConflicts[ConflictID, ResourceID, VotePower]

	// Weight is the Weight of the Conflict.
	Weight *weight.Weight

	// LatestVotes is the set of the latest votes of the Conflict.
	LatestVotes *shrinkingmap.ShrinkingMap[identity.ID, *vote.Vote[VotePower]]

	// AcceptanceStateUpdated is triggered when the AcceptanceState of the Conflict is updated.
	AcceptanceStateUpdated *event.Event2[acceptance.State, acceptance.State]

	// PreferredInsteadUpdated is triggered when the preferred instead value of the Conflict is updated.
	PreferredInsteadUpdated *event.Event1[*Conflict[ConflictID, ResourceID, VotePower]]

	// LikedInsteadAdded is triggered when a liked instead reference is added to the Conflict.
	LikedInsteadAdded *event.Event1[*Conflict[ConflictID, ResourceID, VotePower]]

	// LikedInsteadRemoved is triggered when a liked instead reference is removed from the Conflict.
	LikedInsteadRemoved *event.Event1[*Conflict[ConflictID, ResourceID, VotePower]]

	// childUnhookMethods is a mapping of children to their unhook functions.
	childUnhookMethods *shrinkingmap.ShrinkingMap[ConflictID, func()]

	// preferredInstead is the preferred instead value of the Conflict.
	preferredInstead *Conflict[ConflictID, ResourceID, VotePower]

	// evicted
	evicted atomic.Bool

	// preferredInsteadMutex is used to synchronize access to the preferred instead value of the Conflict.
	preferredInsteadMutex sync.RWMutex

	// likedInstead is the set of liked instead Conflicts.
	likedInstead *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	// likedInsteadSources is a mapping of liked instead Conflicts to the set of parents that inherited them.
	likedInsteadSources *shrinkingmap.ShrinkingMap[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]]

	// likedInsteadMutex is used to synchronize access to the liked instead value of the Conflict.
	likedInsteadMutex sync.RWMutex

	// structureMutex is used to synchronize access to the structure of the Conflict.
	structureMutex sync.RWMutex

	// acceptanceThreshold is the function that is used to retrieve the acceptance threshold of the committee.
	acceptanceThreshold func() int64

	// unhookAcceptanceMonitoring
	unhookAcceptanceMonitoring func()

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// NewConflict creates a new Conflict.
func NewConflict[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](id ConflictID, parents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]], conflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictID, ResourceID, VotePower]], initialWeight *weight.Weight, pendingTasksCounter *syncutils.Counter, acceptanceThresholdProvider func() int64) *Conflict[ConflictID, ResourceID, VotePower] {
	c := &Conflict[ConflictID, ResourceID, VotePower]{
		ID:                      id,
		Parents:                 advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
		Children:                advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
		ConflictSets:            advancedset.New[*ConflictSet[ConflictID, ResourceID, VotePower]](),
		Weight:                  initialWeight,
		LatestVotes:             shrinkingmap.New[identity.ID, *vote.Vote[VotePower]](),
		AcceptanceStateUpdated:  event.New2[acceptance.State, acceptance.State](),
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID, VotePower]](),
		LikedInsteadAdded:       event.New1[*Conflict[ConflictID, ResourceID, VotePower]](),
		LikedInsteadRemoved:     event.New1[*Conflict[ConflictID, ResourceID, VotePower]](),

		childUnhookMethods:  shrinkingmap.New[ConflictID, func()](),
		acceptanceThreshold: acceptanceThresholdProvider,
		likedInstead:        advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
		likedInsteadSources: shrinkingmap.New[ConflictID, *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]](),
	}

	c.preferredInstead = c

	parents.Range(func(parent *Conflict[ConflictID, ResourceID, VotePower]) {
		if c.Parents.Add(parent) {
			parent.registerChild(c)
		}
	})

	c.unhookAcceptanceMonitoring = c.Weight.Validators.OnTotalWeightUpdated.Hook(func(updatedWeight int64) {
		if threshold := c.acceptanceThreshold(); c.IsPending() && updatedWeight >= threshold {
			c.setAcceptanceState(acceptance.Accepted)
		}
	}).Unhook

	// in case the initial weight is enough to accept the conflict, accept it immediately
	if threshold := c.acceptanceThreshold(); initialWeight.Value().ValidatorsWeight() >= threshold {
		c.setAcceptanceState(acceptance.Accepted)
	}

	c.ConflictingConflicts = NewSortedConflicts[ConflictID, ResourceID, VotePower](c, pendingTasksCounter)
	c.JoinConflictSets(conflictSets)

	return c
}

// JoinConflictSets registers the Conflict with the given ConflictSets.
func (c *Conflict[ConflictID, ResourceID, VotePower]) JoinConflictSets(conflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictID, ResourceID, VotePower]]) (joinedConflictSets *advancedset.AdvancedSet[ResourceID], err error) {
	if c.evicted.Load() {
		return nil, xerrors.Errorf("tried to join conflict sets of evicted conflict: %w", conflictdag.ErrEntityEvicted)
	}

	registerConflictingConflict := func(c, conflict *Conflict[ConflictID, ResourceID, VotePower]) {
		c.structureMutex.Lock()
		defer c.structureMutex.Unlock()

		if c.ConflictingConflicts.Add(conflict) {
			if conflict.IsAccepted() {
				c.setAcceptanceState(acceptance.Rejected)
			}
		}
	}

	joinedConflictSets = advancedset.New[ResourceID]()
	conflictSets.Range(func(conflictSet *ConflictSet[ConflictID, ResourceID, VotePower]) {
		if c.ConflictSets.Add(conflictSet) {
			if otherConflicts := conflictSet.Add(c); otherConflicts != nil {
				otherConflicts.Range(func(otherConflict *Conflict[ConflictID, ResourceID, VotePower]) {
					registerConflictingConflict(c, otherConflict)
					registerConflictingConflict(otherConflict, c)
				})

				joinedConflictSets.Add(conflictSet.ID)
			}
		}
	})

	return joinedConflictSets, nil
}

func (c *Conflict[ConflictID, ResourceID, VotePower]) removeParent(parent *Conflict[ConflictID, ResourceID, VotePower]) (removed bool) {
	if removed = c.Parents.Delete(parent); removed {
		parent.unregisterChild(c)
	}

	return removed
}

// UpdateParents updates the parents of the Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) UpdateParents(addedParent *Conflict[ConflictID, ResourceID, VotePower], removedParents *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]) (updated bool) {
	c.structureMutex.Lock()
	defer c.structureMutex.Unlock()

	removedParents.Range(func(removedParent *Conflict[ConflictID, ResourceID, VotePower]) {
		updated = c.removeParent(removedParent) || updated
	})

	if c.Parents.Add(addedParent) {
		addedParent.registerChild(c)

		updated = true
	}

	return updated
}

func (c *Conflict[ConflictID, ResourceID, VotePower]) ApplyVote(vote *vote.Vote[VotePower]) {
	// abort if the conflict has already been accepted or rejected
	if !c.Weight.AcceptanceState().IsPending() {
		return
	}

	// abort if the vote is not relevant (and apply cumulative weight if no validator has made statements yet)
	if !c.isValidatorRelevant(vote.Voter) {
		if c.LatestVotes.IsEmpty() && vote.IsLiked() {
			c.Weight.AddCumulativeWeight(1)
		}

		return
	}

	// abort if we have another vote from the same validator with higher power
	latestVote, exists := c.LatestVotes.Get(vote.Voter)
	if exists && latestVote.Power.Compare(vote.Power) >= 0 {
		return
	}

	// update the latest vote
	c.LatestVotes.Set(vote.Voter, vote)

	// abort if the vote does not change the opinion of the validator
	if exists && latestVote.IsLiked() == vote.IsLiked() {
		return
	}

	if vote.IsLiked() {
		c.Weight.Validators.Add(vote.Voter)
	} else {
		c.Weight.Validators.Delete(vote.Voter)
	}
}

// IsPending returns true if the Conflict is pending.
func (c *Conflict[ConflictID, ResourceID, VotePower]) IsPending() bool {
	return c.Weight.Value().AcceptanceState().IsPending()
}

// IsAccepted returns true if the Conflict is accepted.
func (c *Conflict[ConflictID, ResourceID, VotePower]) IsAccepted() bool {
	return c.Weight.Value().AcceptanceState().IsAccepted()
}

// IsRejected returns true if the Conflict is rejected.
func (c *Conflict[ConflictID, ResourceID, VotePower]) IsRejected() bool {
	return c.Weight.Value().AcceptanceState().IsRejected()
}

// IsPreferred returns true if the Conflict is preferred instead of its conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID, VotePower]) IsPreferred() bool {
	c.preferredInsteadMutex.RLock()
	defer c.preferredInsteadMutex.RUnlock()

	return c.preferredInstead == c
}

// PreferredInstead returns the preferred instead value of the Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) PreferredInstead() *Conflict[ConflictID, ResourceID, VotePower] {
	c.preferredInsteadMutex.RLock()
	defer c.preferredInsteadMutex.RUnlock()

	return c.preferredInstead
}

// IsLiked returns true if the Conflict is liked instead of other conflicting Conflicts.
func (c *Conflict[ConflictID, ResourceID, VotePower]) IsLiked() bool {
	c.likedInsteadMutex.RLock()
	defer c.likedInsteadMutex.RUnlock()

	return c.IsPreferred() && c.likedInstead.IsEmpty()
}

// LikedInstead returns the set of liked instead Conflicts.
func (c *Conflict[ConflictID, ResourceID, VotePower]) LikedInstead() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]] {
	c.likedInsteadMutex.RLock()
	defer c.likedInsteadMutex.RUnlock()

	return c.likedInstead.Clone()
}

// Evict cleans up the sortedConflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) Evict() (evictedConflicts []ConflictID, err error) {
	if firstEvictCall := !c.evicted.Swap(true); !firstEvictCall {
		return nil, nil
	}

	c.unhookAcceptanceMonitoring()

	switch c.Weight.AcceptanceState() {
	case acceptance.Pending:
		return nil, xerrors.Errorf("tried to evict pending conflict with %s: %w", c.ID, conflictdag.ErrFatal)
	case acceptance.Accepted:
		// remove evicted conflict from parents of children (merge to master)
		c.Children.Range(func(childConflict *Conflict[ConflictID, ResourceID, VotePower]) {
			childConflict.structureMutex.Lock()
			defer childConflict.structureMutex.Unlock()

			childConflict.removeParent(c)
		})
	case acceptance.Rejected:
		// evict the entire future cone of rejected conflicts
		if err = c.Children.ForEach(func(childConflict *Conflict[ConflictID, ResourceID, VotePower]) (err error) {
			evictedChildConflicts, err := childConflict.Evict()
			if err != nil {
				return xerrors.Errorf("failed to evict child conflict %s: %w", childConflict.ID, err)
			}

			evictedConflicts = append(evictedConflicts, evictedChildConflicts...)

			return nil
		}); err != nil {
			return nil, err
		}
	}

	c.structureMutex.Lock()
	defer c.structureMutex.Unlock()

	c.Parents.Range(func(parentConflict *Conflict[ConflictID, ResourceID, VotePower]) {
		parentConflict.unregisterChild(c)
	})
	c.Parents.Clear()

	c.ConflictSets.Range(func(conflictSet *ConflictSet[ConflictID, ResourceID, VotePower]) {
		conflictSet.Remove(c)
	})
	c.ConflictSets.Clear()

	for _, conflict := range c.ConflictingConflicts.Shutdown() {
		if conflict != c {
			conflict.ConflictingConflicts.Remove(c.ID)
			c.ConflictingConflicts.Remove(conflict.ID)

			if c.IsAccepted() {
				evictedChildConflicts, err := conflict.Evict()
				if err != nil {
					return nil, xerrors.Errorf("failed to evict child conflict %s: %w", conflict.ID, err)
				}

				evictedConflicts = append(evictedConflicts, evictedChildConflicts...)
			}
		}
	}

	c.ConflictingConflicts.Remove(c.ID)
	evictedConflicts = append(evictedConflicts, c.ID)

	return evictedConflicts, nil
}

// Compare compares the Conflict to the given other Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) Compare(other *Conflict[ConflictID, ResourceID, VotePower]) int {
	// no need to lock a mutex here, because the Weight is already thread-safe

	if c == other {
		return weight.Equal
	}

	if other == nil {
		return weight.Heavier
	}

	if c == nil {
		return weight.Lighter
	}

	if result := c.Weight.Compare(other.Weight); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(c.ID.Bytes()), lo.PanicOnErr(other.ID.Bytes()))
}

// String returns a human-readable representation of the Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) String() string {
	// no need to lock a mutex here, because the Weight is already thread-safe

	return stringify.Struct("Conflict",
		stringify.NewStructField("id", c.ID),
		stringify.NewStructField("weight", c.Weight),
	)
}

// registerChild registers the given child Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) registerChild(child *Conflict[ConflictID, ResourceID, VotePower]) {
	c.structureMutex.Lock()
	defer c.structureMutex.Unlock()

	if c.Children.Add(child) {
		// hold likedInsteadMutex while determining our liked instead state
		c.likedInsteadMutex.Lock()
		defer c.likedInsteadMutex.Unlock()

		c.childUnhookMethods.Set(child.ID, lo.Batch(
			c.AcceptanceStateUpdated.Hook(func(_, newState acceptance.State) {
				if newState.IsRejected() {
					child.setAcceptanceState(newState)
				}
			}).Unhook,

			c.LikedInsteadRemoved.Hook(func(reference *Conflict[ConflictID, ResourceID, VotePower]) {
				child.removeInheritedLikedInsteadReference(c, reference)
			}).Unhook,

			c.LikedInsteadAdded.Hook(func(conflict *Conflict[ConflictID, ResourceID, VotePower]) {
				child.structureMutex.Lock()
				defer child.structureMutex.Unlock()

				child.addInheritedLikedInsteadReference(c, conflict)
			}).Unhook,
		))

		for conflicts := c.likedInstead.Iterator(); conflicts.HasNext(); {
			child.addInheritedLikedInsteadReference(c, conflicts.Next())
		}

		if c.IsRejected() {
			child.setAcceptanceState(acceptance.Rejected)
		}
	}
}

// unregisterChild unregisters the given child Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) unregisterChild(conflict *Conflict[ConflictID, ResourceID, VotePower]) {
	c.structureMutex.Lock()
	defer c.structureMutex.Unlock()

	if c.Children.Delete(conflict) {
		if unhookFunc, exists := c.childUnhookMethods.Get(conflict.ID); exists {
			c.childUnhookMethods.Delete(conflict.ID)

			unhookFunc()
		}
	}
}

// addInheritedLikedInsteadReference adds the given reference as a liked instead reference from the given source.
func (c *Conflict[ConflictID, ResourceID, VotePower]) addInheritedLikedInsteadReference(source, reference *Conflict[ConflictID, ResourceID, VotePower]) {
	c.likedInsteadMutex.Lock()
	defer c.likedInsteadMutex.Unlock()

	// abort if the source already added the reference or if the source already existed
	if sources := lo.Return1(c.likedInsteadSources.GetOrCreate(reference.ID, lo.NoVariadic(advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]]))); !sources.Add(source) || !c.likedInstead.Add(reference) {
		return
	}

	// remove the "preferred instead reference" (that might have been set as a default)
	if preferredInstead := c.PreferredInstead(); c.likedInstead.Delete(preferredInstead) {
		c.LikedInsteadRemoved.Trigger(preferredInstead)
	}

	// trigger within the scope of the lock to ensure the correct queueing order
	c.LikedInsteadAdded.Trigger(reference)
}

// removeInheritedLikedInsteadReference removes the given reference as a liked instead reference from the given source.
func (c *Conflict[ConflictID, ResourceID, VotePower]) removeInheritedLikedInsteadReference(source, reference *Conflict[ConflictID, ResourceID, VotePower]) {
	c.likedInsteadMutex.Lock()
	defer c.likedInsteadMutex.Unlock()

	// abort if the reference did not exist
	if sources, sourcesExist := c.likedInsteadSources.Get(reference.ID); !sourcesExist || !sources.Delete(source) || !sources.IsEmpty() || !c.likedInsteadSources.Delete(reference.ID) || !c.likedInstead.Delete(reference) {
		return
	}

	// trigger within the scope of the lock to ensure the correct queueing order
	c.LikedInsteadRemoved.Trigger(reference)

	// fall back to preferred instead if not preferred and parents are liked
	if preferredInstead := c.PreferredInstead(); c.likedInstead.IsEmpty() && preferredInstead != c {
		c.likedInstead.Add(preferredInstead)

		// trigger within the scope of the lock to ensure the correct queueing order
		c.LikedInsteadAdded.Trigger(preferredInstead)
	}
}

// setPreferredInstead sets the preferred instead value of the Conflict.
func (c *Conflict[ConflictID, ResourceID, VotePower]) setPreferredInstead(preferredInstead *Conflict[ConflictID, ResourceID, VotePower]) (previousPreferredInstead *Conflict[ConflictID, ResourceID, VotePower]) {
	c.likedInsteadMutex.Lock()
	defer c.likedInsteadMutex.Unlock()

	if func() (updated bool) {
		c.preferredInsteadMutex.Lock()
		defer c.preferredInsteadMutex.Unlock()

		if previousPreferredInstead, updated = c.preferredInstead, previousPreferredInstead != preferredInstead; updated {
			c.preferredInstead = preferredInstead
			c.PreferredInsteadUpdated.Trigger(preferredInstead)
		}

		return updated
	}() {
		if c.likedInstead.Delete(previousPreferredInstead) {
			// trigger within the scope of the lock to ensure the correct queueing order
			c.LikedInsteadRemoved.Trigger(previousPreferredInstead)
		}

		if !c.IsPreferred() && c.likedInstead.IsEmpty() {
			c.likedInstead.Add(preferredInstead)

			// trigger within the scope of the lock to ensure the correct queueing order
			c.LikedInsteadAdded.Trigger(preferredInstead)
		}
	}

	return previousPreferredInstead
}

// setAcceptanceState sets the acceptance state of the Conflict and returns the previous acceptance state (it triggers
// an AcceptanceStateUpdated event if the acceptance state was updated).
func (c *Conflict[ConflictID, ResourceID, VotePower]) setAcceptanceState(newState acceptance.State) (previousState acceptance.State) {
	if previousState = c.Weight.SetAcceptanceState(newState); previousState == newState {
		return previousState
	}

	// propagate acceptance to parents first
	if newState.IsAccepted() {
		c.Parents.Range(func(parent *Conflict[ConflictID, ResourceID, VotePower]) {
			parent.setAcceptanceState(acceptance.Accepted)
		})
	}

	c.AcceptanceStateUpdated.Trigger(previousState, newState)

	return previousState
}

func (c *Conflict[ConflictID, ResourceID, VotePower]) isValidatorRelevant(id identity.ID) bool {
	validatorWeight, exists := c.Weight.Validators.Weights.Get(id)

	return exists && validatorWeight.Value > 0
}
