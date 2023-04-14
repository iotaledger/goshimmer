package newconflictdag

import (
	"sync"

	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

// ConflictDAG represents a data structure that tracks causal relationships between Conflicts and that allows to
// efficiently manage these Conflicts (and vote on their fate).
type ConflictDAG[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] struct {
	// Events contains the Events of the ConflictDAG.
	Events *Events[ConflictID, ResourceID]

	// acceptanceThresholdProvider is the function that is used to retrieve the acceptance threshold of the committee.
	acceptanceThresholdProvider func() int64

	// conflictsByID is a mapping of ConflictIDs to Conflicts.
	conflictsByID *shrinkingmap.ShrinkingMap[ConflictID, *Conflict[ConflictID, ResourceID, VotePower]]

	conflictUnhooks *shrinkingmap.ShrinkingMap[ConflictID, func()]

	// conflictSetsByID is a mapping of ResourceIDs to ConflictSets.
	conflictSetsByID *shrinkingmap.ShrinkingMap[ResourceID, *ConflictSet[ConflictID, ResourceID, VotePower]]

	// pendingTasks is a counter that keeps track of the number of pending tasks.
	pendingTasks *syncutils.Counter

	// mutex is used to synchronize access to the ConflictDAG.
	mutex sync.RWMutex
}

// New creates a new ConflictDAG.
func New[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]](acceptanceThresholdProvider func() int64) *ConflictDAG[ConflictID, ResourceID, VotePower] {
	return &ConflictDAG[ConflictID, ResourceID, VotePower]{

		acceptanceThresholdProvider: acceptanceThresholdProvider,
		conflictsByID:               shrinkingmap.New[ConflictID, *Conflict[ConflictID, ResourceID, VotePower]](),
		conflictUnhooks:             shrinkingmap.New[ConflictID, func()](),
		conflictSetsByID:            shrinkingmap.New[ResourceID, *ConflictSet[ConflictID, ResourceID, VotePower]](),
		pendingTasks:                syncutils.NewCounter(),
	}
}

// CreateConflict creates a new Conflict that is conflicting over the given ResourceIDs and that has the given parents.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) error {
	err := func() error {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		parents, err := c.conflicts(parentIDs, !initialWeight.AcceptanceState().IsRejected())
		if err != nil {
			return xerrors.Errorf("failed to create conflict: %w", err)
		}

		conflictSets, err := c.conflictSets(resourceIDs, !initialWeight.AcceptanceState().IsRejected())
		if err != nil {
			return xerrors.Errorf("failed to create conflict: %w", err)
		}

		if _, isNew := c.conflictsByID.GetOrCreate(id, func() *Conflict[ConflictID, ResourceID, VotePower] {
			newConflict := NewConflict[ConflictID, ResourceID, VotePower](id, parents, conflictSets, initialWeight, c.pendingTasks, c.acceptanceThresholdProvider)

			// attach to the acceptance state updated event and propagate that event to the outside.
			// also need to remember the unhook method to properly evict the conflict.
			c.conflictUnhooks.Set(id, newConflict.AcceptanceStateUpdated.Hook(func(oldState, newState acceptance.State) {
				if newState.IsAccepted() {
					c.Events.ConflictAccepted.Trigger(newConflict.ID)
					return
				}
				if newState.IsRejected() {
					c.Events.ConflictRejected.Trigger(newConflict.ID)
				}
			}).Unhook)

			return newConflict
		}); !isNew {
			return xerrors.Errorf("tried to create conflict with %s twice: %w", id, ErrConflictExists)
		}

		return nil
	}()

	if err == nil {
		c.Events.ConflictCreated.Trigger(id)
	}

	return err
}

// ReadConsistent write locks the ConflictDAG and exposes read-only methods to the callback to perform multiple reads while maintaining the same ConflictDAG state.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) ReadConsistent(callback func(conflictDAG ReadLockedConflictDAG[ConflictID, ResourceID, VotePower]) error) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	return callback(c)
}

// JoinConflictSets adds the Conflict to the given ConflictSets and returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) JoinConflictSets(conflictID ConflictID, resourceIDs ...ResourceID) error {
	joinedConflictSets, err := func() ([]ResourceID, error) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			return nil, xerrors.Errorf("tried to modify evicted conflict with %s: %w", conflictID, ErrEntityEvicted)
		}

		conflictSets, err := c.conflictSets(resourceIDs, !currentConflict.IsRejected())
		if err != nil {
			return nil, xerrors.Errorf("failed to join conflict sets: %w", err)
		}

		joinedConflictSets, err := currentConflict.JoinConflictSets(conflictSets...)
		if err != nil {
			return nil, xerrors.Errorf("failed to join conflict sets: %w", err)
		}

		return joinedConflictSets, nil
	}()
	if err != nil {
		return err
	}

	if len(joinedConflictSets) > 0 {
		c.Events.ConflictingResourcesAdded.Trigger(conflictID, joinedConflictSets)
	}

	return nil
}

// UpdateConflictParents updates the parents of the given Conflict and returns an error if the operation failed.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs ...ConflictID) error {
	updated, err := func() (bool, error) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, currentConflictExists := c.conflictsByID.Get(conflictID)
		if !currentConflictExists {
			return false, xerrors.Errorf("tried to modify evicted conflict with %s: %w", conflictID, ErrEntityEvicted)
		}

		addedParent, addedParentExists := c.conflictsByID.Get(addedParentID)
		if !addedParentExists {
			if !currentConflict.IsRejected() {
				// UpdateConflictParents is only called when a Conflict is forked, which means that the added parent
				// must exist (unless it was forked on top of a rejected branch, just before eviction).
				return false, xerrors.Errorf("tried to add non-existent parent with %s: %w", addedParentID, ErrFatal)
			}

			return false, xerrors.Errorf("tried to add evicted parent with %s to rejected conflict with %s: %w", addedParentID, conflictID, ErrEntityEvicted)
		}

		removedParents, err := c.conflicts(removedParentIDs, !currentConflict.IsRejected())
		if err != nil {
			return false, xerrors.Errorf("failed to update conflict parents: %w", err)
		}

		return currentConflict.UpdateParents(addedParent, removedParents...), nil
	}()
	if err != nil {
		return err
	}

	if updated {
		c.Events.ConflictParentsUpdated.Trigger(conflictID, addedParentID, removedParentIDs)
	}

	return nil
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) LikedInstead(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID] {
	c.pendingTasks.WaitIsZero()

	likedInstead := advancedset.New[ConflictID]()
	for _, conflictID := range conflictIDs {
		if currentConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if likedConflict := heaviestConflict(currentConflict.LikedInstead()); likedConflict != nil {
				likedInstead.Add(likedConflict.ID)
			}
		}
	}

	return likedInstead
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) FutureCone(conflictIDs *advancedset.AdvancedSet[ConflictID]) (futureCone *advancedset.AdvancedSet[ConflictID]) {
	futureCone = advancedset.New[ConflictID]()
	for futureConeWalker := walker.New[*Conflict[ConflictID, ResourceID, VotePower]]().PushAll(lo.Return1(c.conflicts(conflictIDs.Slice(), true))...); futureConeWalker.HasNext(); {
		if conflict := futureConeWalker.Next(); futureCone.Add(conflict.ID) {
			futureConeWalker.PushAll(conflict.Children.Slice()...)
		}
	}

	return futureCone
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) ConflictingConflicts(conflictID ConflictID) (conflictingConflicts *advancedset.AdvancedSet[ConflictID], exists bool) {
	conflict, exists := c.conflictsByID.Get(conflictID)
	if !exists {
		return nil, false
	}

	conflictingConflicts = advancedset.New[ConflictID]()
	_ = conflict.ConflictingConflicts.ForEach(func(conflictingConflict *Conflict[ConflictID, ResourceID, VotePower]) error {
		conflictingConflicts.Add(conflictingConflict.ID())
		return nil
	})

	return conflictingConflicts, true
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) AllConflictsSupported(issuerID identity.ID, conflictIDs ...ConflictID) bool {
	for _, conflict := range lo.Return1(c.conflicts(conflictIDs, true)) {
		if lastVote, exists := conflict.LatestVotes.Get(issuerID); !exists || !lastVote.IsLiked() {
			return false
		}
	}

	return true
}

// CastVotes applies the given votes to the ConflictDAG.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CastVotes(vote *vote.Vote[VotePower], conflictIDs ...ConflictID) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	// TODO: introduce a DAG mutex to lock per identity when casting a vote

	supportedConflicts, revokedConflicts, err := c.determineVotes(conflictIDs...)
	if err != nil {
		return xerrors.Errorf("failed to determine votes: %w", err)
	}

	for supportedConflict := supportedConflicts.Iterator(); supportedConflict.HasNext(); {
		supportedConflict.Next().ApplyVote(vote.WithLiked(true))
	}

	for revokedConflict := revokedConflicts.Iterator(); revokedConflict.HasNext(); {
		revokedConflict.Next().ApplyVote(vote.WithLiked(false))
	}

	return nil
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) AcceptanceState(conflictIDs ...ConflictID) acceptance.State {
	// we are on master reality.
	if len(conflictIDs) == 0 {
		return acceptance.Accepted
	}

	lowestObservedState := acceptance.Accepted
	for _, conflictID := range conflictIDs {
		conflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			panic(xerrors.Errorf("tried to retrieve non-existing conflict: %w", ErrFatal))
		}

		if lowestObservedState == acceptance.Accepted && conflict.IsPending() {
			lowestObservedState = acceptance.Pending
		}

		if conflict.IsRejected() {
			return acceptance.Rejected
		}
	}

	return lowestObservedState
}

// UnacceptedConflicts takes a set of ConflictIDs and removes all the accepted Conflicts (leaving only the
// pending or rejected ones behind).
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) UnacceptedConflicts(conflictIDs ...ConflictID) *advancedset.AdvancedSet[ConflictID] {
	// TODO: introduce optsMergeToMaster
	//if !c.optsMergeToMaster {
	//	return conflictIDs.Clone()
	//}

	pendingConflictIDs := advancedset.New[ConflictID]()
	for _, currentConflictID := range conflictIDs {
		if conflict, exists := c.conflictsByID.Get(currentConflictID); exists && !conflict.IsAccepted() {
			pendingConflictIDs.Add(currentConflictID)
		}
	}

	return pendingConflictIDs
}

// EvictConflict removes conflict with given ConflictID from ConflictDAG.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) EvictConflict(conflictID ConflictID) error {
	evictedConflictIDs, err := func() ([]ConflictID, error) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		// evicting an already evicted conflict is fine
		conflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			return nil, nil
		}

		// abort if we faced an error while evicting the conflict
		evictedConflictIDs, err := conflict.Evict()
		if err != nil {
			return nil, xerrors.Errorf("failed to evict conflict with %s: %w", conflictID, err)
		}

		// remove the conflicts from the ConflictDAG dictionary
		for _, evictedConflictID := range evictedConflictIDs {
			c.conflictsByID.Delete(evictedConflictID)
		}

		// unhook the conflict events and remove the unhook method from the storage
		unhookFunc, unhookExists := c.conflictUnhooks.Get(conflictID)
		if unhookExists {
			unhookFunc()
			c.conflictUnhooks.Delete(conflictID)
		}

		return evictedConflictIDs, nil
	}()
	if err != nil {
		return xerrors.Errorf("failed to evict conflict with %s: %w", conflictID, err)
	}

	// trigger the ConflictEvicted event
	for _, evictedConflictID := range evictedConflictIDs {
		c.Events.ConflictEvicted.Trigger(evictedConflictID)
	}

	return nil
}

// conflicts returns the Conflicts that are associated with the given ConflictIDs. If ignoreMissing is set to true, it
// will ignore missing Conflicts instead of returning an ErrEntityEvicted error.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) conflicts(ids []ConflictID, ignoreMissing bool) ([]*Conflict[ConflictID, ResourceID, VotePower], error) {
	conflicts := make(map[ConflictID]*Conflict[ConflictID, ResourceID, VotePower])
	for _, id := range ids {
		existingConflict, exists := c.conflictsByID.Get(id)
		if !exists {
			if !ignoreMissing {
				return nil, xerrors.Errorf("tried to retrieve an evicted conflict with %s: %w", id, ErrEntityEvicted)
			}

			continue
		}

		conflicts[id] = existingConflict
	}

	return lo.Values(conflicts), nil
}

// conflictSets returns the ConflictSets that are associated with the given ResourceIDs. If createMissing is set to
// true, it will create an empty ConflictSet for each missing ResourceID.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) conflictSets(resourceIDs []ResourceID, createMissing bool) ([]*ConflictSet[ConflictID, ResourceID, VotePower], error) {
	conflictSetsMap := make(map[ResourceID]*ConflictSet[ConflictID, ResourceID, VotePower])
	for _, resourceID := range resourceIDs {
		if conflictSet, exists := c.conflictSetsByID.Get(resourceID); exists {
			conflictSetsMap[resourceID] = conflictSet

			continue
		}

		if !createMissing {
			return nil, xerrors.Errorf("tried to create a Conflict with evicted Resource: %w", ErrEntityEvicted)
		}

		conflictSetsMap[resourceID] = lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *ConflictSet[ConflictID, ResourceID, VotePower] {
			// TODO: hook to conflictSet event that is triggered when it becomes empty
			return NewConflictSet[ConflictID, ResourceID, VotePower](resourceID)
		}))
	}

	return lo.Values(conflictSetsMap), nil
}

// determineVotes determines the Conflicts that are supported and revoked by the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) determineVotes(conflictIDs ...ConflictID) (supportedConflicts, revokedConflicts *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]], err error) {
	supportedConflicts = advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]]()
	revokedConflicts = advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]]()

	revokedWalker := walker.New[*Conflict[ConflictID, ResourceID, VotePower]]()
	revokeConflict := func(revokedConflict *Conflict[ConflictID, ResourceID, VotePower]) error {
		if revokedConflicts.Add(revokedConflict) {
			if supportedConflicts.Has(revokedConflict) {
				return xerrors.Errorf("applied conflicting votes (%s is supported and revoked)", revokedConflict.ID)
			}

			revokedWalker.PushAll(revokedConflict.Children.Slice()...)
		}

		return nil
	}

	supportedWalker := walker.New[*Conflict[ConflictID, ResourceID, VotePower]]()
	supportConflict := func(supportedConflict *Conflict[ConflictID, ResourceID, VotePower]) error {
		if supportedConflicts.Add(supportedConflict) {
			if err := supportedConflict.ConflictingConflicts.ForEach(revokeConflict); err != nil {
				return xerrors.Errorf("failed to collect conflicting conflicts: %w", err)
			}

			supportedWalker.PushAll(supportedConflict.Parents.Slice()...)
		}

		return nil
	}

	for supportedWalker.PushAll(lo.Return1(c.conflicts(conflictIDs, true))...); supportedWalker.HasNext(); {
		if err := supportConflict(supportedWalker.Next()); err != nil {
			return nil, nil, xerrors.Errorf("failed to collect supported conflicts: %w", err)
		}
	}

	for revokedWalker.HasNext() {
		if revokedConflict := revokedWalker.Next(); revokedConflicts.Add(revokedConflict) {
			revokedWalker.PushAll(revokedConflict.Children.Slice()...)
		}
	}

	return supportedConflicts, revokedConflicts, nil
}
