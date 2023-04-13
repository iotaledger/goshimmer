package newconflictdag

import (
	"sync"

	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/vote"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
)

// ConflictDAG represents a data structure that tracks causal relationships between Conflicts and that allows to
// efficiently manage these Conflicts (and vote on their fate).
type ConflictDAG[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] struct {
	// ConflictCreated is triggered when a new Conflict is created.
	ConflictCreated *event.Event1[ConflictID]

	// ConflictingResourcesAdded is triggered when the Conflict is added to a new ConflictSet.
	ConflictingResourcesAdded *event.Event2[ConflictID, []ResourceID]

	// ConflictParentsUpdated is triggered when the parents of a Conflict are updated.
	ConflictParentsUpdated *event.Event3[*Conflict[ConflictID, ResourceID, VotePower], *Conflict[ConflictID, ResourceID, VotePower], []*Conflict[ConflictID, ResourceID, VotePower]]

	// acceptanceThresholdProvider is the function that is used to retrieve the acceptance threshold of the committee.
	acceptanceThresholdProvider func() int64

	// conflictsByID is a mapping of ConflictIDs to Conflicts.
	conflictsByID *shrinkingmap.ShrinkingMap[ConflictID, *Conflict[ConflictID, ResourceID, VotePower]]

	acceptanceHooks *shrinkingmap.ShrinkingMap[ConflictID, *event.Hook[func(int64)]]

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
		ConflictCreated:             event.New1[ConflictID](),
		ConflictingResourcesAdded:   event.New2[ConflictID, []ResourceID](),
		ConflictParentsUpdated:      event.New3[*Conflict[ConflictID, ResourceID, VotePower], *Conflict[ConflictID, ResourceID, VotePower], []*Conflict[ConflictID, ResourceID, VotePower]](),
		acceptanceThresholdProvider: acceptanceThresholdProvider,
		conflictsByID:               shrinkingmap.New[ConflictID, *Conflict[ConflictID, ResourceID, VotePower]](),
		acceptanceHooks:             shrinkingmap.New[ConflictID, *event.Hook[func(int64)]](),
		conflictSetsByID:            shrinkingmap.New[ResourceID, *ConflictSet[ConflictID, ResourceID, VotePower]](),
		pendingTasks:                syncutils.NewCounter(),
	}
}

// CreateConflict creates a new Conflict that is conflicting over the given ResourceIDs and that has the given parents.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) error {
	err := func() error {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		parents := lo.Values(c.Conflicts(parentIDs...))

		conflictSets, err := c.conflictSets(!initialWeight.AcceptanceState().IsRejected(), resourceIDs...)
		if err != nil {
			return xerrors.Errorf("failed to create conflict: %w", err)
		}

		if _, isNew := c.conflictsByID.GetOrCreate(id, func() *Conflict[ConflictID, ResourceID, VotePower] {
			return NewConflict[ConflictID, ResourceID, VotePower](id, parents, conflictSets, initialWeight, c.pendingTasks, c.acceptanceThresholdProvider)
		}); isNew {
			return nil
		}

		return xerrors.Errorf("tried to create conflict with %s twice: %w", id, ErrFatal)
	}()

	c.ConflictCreated.Trigger(id)

	return err
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) conflictSets(createMissing bool, resourceIDs ...ResourceID) ([]*ConflictSet[ConflictID, ResourceID, VotePower], error) {
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

// JoinConflictSets adds the Conflict to the given ConflictSets and returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) JoinConflictSets(conflictID ConflictID, resourceIDs ...ResourceID) (joinedConflictSets []ResourceID, err error) {
	joinedConflictSets, err = func() ([]ResourceID, error) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			return nil, xerrors.Errorf("tried to modify evicted conflict with %s: %w", conflictID, ErrEntityEvicted)
		}

		conflictSets, err := c.conflictSets(!currentConflict.IsRejected(), resourceIDs...)
		if err != nil {
			return nil, xerrors.Errorf("failed to join conflict: %w", err)
		}

		return currentConflict.JoinConflictSets(conflictSets...)
	}()

	if err != nil {
		return nil, err
	}

	if len(joinedConflictSets) > 0 {
		c.ConflictingResourcesAdded.Trigger(conflictID, joinedConflictSets)
	}

	return joinedConflictSets, nil
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs ...ConflictID) bool {
	currentConflict, addedParent, removedParents, updated := func() (*Conflict[ConflictID, ResourceID, VotePower], *Conflict[ConflictID, ResourceID, VotePower], []*Conflict[ConflictID, ResourceID, VotePower], bool) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, currentConflictExists := c.conflictsByID.Get(conflictID)
		if !currentConflictExists {
			return nil, nil, nil, false
		}

		addedParent, addedParentExists := c.Conflicts(addedParentID)[addedParentID]
		if !addedParentExists {
			return nil, nil, nil, false
		}

		removedParents := lo.Values(c.Conflicts(removedParentIDs...))

		return currentConflict, addedParent, removedParents, currentConflict.UpdateParents(addedParent, removedParents...)
	}()

	if updated {
		c.ConflictParentsUpdated.Trigger(currentConflict, addedParent, removedParents)
	}

	return updated
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) LikedInstead(conflictIDs ...ConflictID) map[ConflictID]*Conflict[ConflictID, ResourceID, VotePower] {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	likedInstead := make(map[ConflictID]*Conflict[ConflictID, ResourceID, VotePower])
	for _, conflictID := range conflictIDs {
		if currentConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if likedConflict := heaviestConflict(currentConflict.LikedInstead()); likedConflict != nil {
				likedInstead[likedConflict.ID] = likedConflict
			}
		}
	}

	return likedInstead
}

// Conflicts returns the Conflicts that are associated with the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) Conflicts(ids ...ConflictID) map[ConflictID]*Conflict[ConflictID, ResourceID, VotePower] {
	conflicts := make(map[ConflictID]*Conflict[ConflictID, ResourceID, VotePower])
	for _, id := range ids {
		if existingConflict, exists := c.conflictsByID.Get(id); exists {
			conflicts[id] = existingConflict
		}
	}

	return conflicts
}

// ConflictSets returns the ConflictSets that are associated with the given ResourceIDs.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) ConflictSets(resourceIDs ...ResourceID) map[ResourceID]*ConflictSet[ConflictID, ResourceID, VotePower] {
	conflictSets := make(map[ResourceID]*ConflictSet[ConflictID, ResourceID, VotePower])
	for _, resourceID := range resourceIDs {
		conflictSets[resourceID] = lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *ConflictSet[ConflictID, ResourceID, VotePower] {
			// TODO: hook to conflictSet event that is triggered when it becomes empty
			return NewConflictSet[ConflictID, ResourceID, VotePower](resourceID)
		}))
	}

	return conflictSets
}

// CastVotes applies the given votes to the ConflictDAG.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CastVotes(vote *vote.Vote[VotePower], conflictIDs ...ConflictID) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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

// EvictConflict removes conflict with given ConflictID from ConflictDAG.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) EvictConflict(conflictID ConflictID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// TODO: make locking more fine-grained on conflictset level

	conflict, exists := c.conflictsByID.Get(conflictID)
	if !exists {
		return
	}

	evictedConflictIDs := conflict.Evict()

	for _, evictedConflictID := range evictedConflictIDs {
		c.conflictsByID.Delete(evictedConflictID)
	}

	// _ = conflictSets.ForEach(func(conflictSet *ConflictSet[ConflictID, ResourceID, VotePower]) (err error) {
	//	if conflictSet.members.IsEmpty() {
	//		c.conflictSetsByID.Delete(conflictSet.ID)
	//	}
	//
	//	return nil
	// })
}

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

	for supportedWalker.PushAll(lo.Values(c.Conflicts(conflictIDs...))...); supportedWalker.HasNext(); {
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
