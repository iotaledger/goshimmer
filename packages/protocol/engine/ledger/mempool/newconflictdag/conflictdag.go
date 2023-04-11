package newconflictdag

import (
	"sync"

	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/conflict"
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
type ConflictDAG[ConflictID, ResourceID conflict.IDType, VotePower constraints.Comparable[VotePower]] struct {
	// ConflictCreated is triggered when a new Conflict is created.
	ConflictCreated *event.Event1[*conflict.Conflict[ConflictID, ResourceID, VotePower]]

	// ConflictingResourcesAdded is triggered when the Conflict is added to a new ConflictSet.
	ConflictingResourcesAdded *event.Event2[*conflict.Conflict[ConflictID, ResourceID, VotePower], map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower]]

	// ConflictParentsUpdated is triggered when the parents of a Conflict are updated.
	ConflictParentsUpdated *event.Event3[*conflict.Conflict[ConflictID, ResourceID, VotePower], *conflict.Conflict[ConflictID, ResourceID, VotePower], []*conflict.Conflict[ConflictID, ResourceID, VotePower]]

	// totalWeightProvider is the function that is used to retrieve the total weight of the network.
	totalWeightProvider func() int64

	// conflictsByID is a mapping of ConflictIDs to Conflicts.
	conflictsByID *shrinkingmap.ShrinkingMap[ConflictID, *conflict.Conflict[ConflictID, ResourceID, VotePower]]

	acceptanceHooks *shrinkingmap.ShrinkingMap[ConflictID, *event.Hook[func(int64)]]

	// conflictSetsByID is a mapping of ResourceIDs to ConflictSets.
	conflictSetsByID *shrinkingmap.ShrinkingMap[ResourceID, *conflict.Set[ConflictID, ResourceID, VotePower]]

	// pendingTasks is a counter that keeps track of the number of pending tasks.
	pendingTasks *syncutils.Counter

	// mutex is used to synchronize access to the ConflictDAG.
	mutex sync.RWMutex
}

// New creates a new ConflictDAG.
func New[ConflictID, ResourceID conflict.IDType, VotePower constraints.Comparable[VotePower]](totalWeightProvider func() int64) *ConflictDAG[ConflictID, ResourceID, VotePower] {
	return &ConflictDAG[ConflictID, ResourceID, VotePower]{
		ConflictCreated:           event.New1[*conflict.Conflict[ConflictID, ResourceID, VotePower]](),
		ConflictingResourcesAdded: event.New2[*conflict.Conflict[ConflictID, ResourceID, VotePower], map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower]](),
		ConflictParentsUpdated:    event.New3[*conflict.Conflict[ConflictID, ResourceID, VotePower], *conflict.Conflict[ConflictID, ResourceID, VotePower], []*conflict.Conflict[ConflictID, ResourceID, VotePower]](),
		totalWeightProvider:       totalWeightProvider,
		conflictsByID:             shrinkingmap.New[ConflictID, *conflict.Conflict[ConflictID, ResourceID, VotePower]](),
		acceptanceHooks:           shrinkingmap.New[ConflictID, *event.Hook[func(int64)]](),
		conflictSetsByID:          shrinkingmap.New[ResourceID, *conflict.Set[ConflictID, ResourceID, VotePower]](),
		pendingTasks:              syncutils.NewCounter(),
	}
}

const bftThreshold = 0.67

// CreateConflict creates a new Conflict that is conflicting over the given ResourceIDs and that has the given parents.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CreateConflict(id ConflictID, parentIDs []ConflictID, resourceIDs []ResourceID, initialWeight *weight.Weight) *conflict.Conflict[ConflictID, ResourceID, VotePower] {
	createdConflict := func() *conflict.Conflict[ConflictID, ResourceID, VotePower] {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		parents := lo.Values(c.Conflicts(parentIDs...))
		conflictSets := lo.Values(c.ConflictSets(resourceIDs...))

		if createdConflict, isNew := c.conflictsByID.GetOrCreate(id, func() *conflict.Conflict[ConflictID, ResourceID, VotePower] {
			return conflict.New[ConflictID, ResourceID, VotePower](id, parents, conflictSets, initialWeight, c.pendingTasks)
		}); isNew {
			c.acceptanceHooks.Set(createdConflict.ID, createdConflict.Weight.Validators.OnTotalWeightUpdated.Hook(func(updatedWeight int64) {
				if createdConflict.Weight.AcceptanceState().IsPending() && updatedWeight > int64(float64(c.totalWeightProvider())*bftThreshold) {
					createdConflict.SetAcceptanceState(acceptance.Accepted)
				}
			}))

			return createdConflict
		}

		panic("tried to re-create an already existing conflict")
	}()

	c.ConflictCreated.Trigger(createdConflict)

	return createdConflict
}

// JoinConflictSets adds the Conflict to the given ConflictSets and returns true if the conflict membership was modified during this operation.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) JoinConflictSets(conflictID ConflictID, resourceIDs ...ResourceID) (joinedConflictSets map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower]) {
	currentConflict, joinedConflictSets := func() (*conflict.Conflict[ConflictID, ResourceID, VotePower], map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower]) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, exists := c.conflictsByID.Get(conflictID)
		if !exists {
			return nil, nil
		}

		return currentConflict, currentConflict.JoinConflictSets(lo.Values(c.ConflictSets(resourceIDs...))...)
	}()

	if len(joinedConflictSets) > 0 {
		c.ConflictingResourcesAdded.Trigger(currentConflict, joinedConflictSets)
	}

	return joinedConflictSets
}

func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) UpdateConflictParents(conflictID ConflictID, addedParentID ConflictID, removedParentIDs ...ConflictID) bool {
	currentConflict, addedParent, removedParents, updated := func() (*conflict.Conflict[ConflictID, ResourceID, VotePower], *conflict.Conflict[ConflictID, ResourceID, VotePower], []*conflict.Conflict[ConflictID, ResourceID, VotePower], bool) {
		c.mutex.RLock()
		defer c.mutex.RUnlock()

		currentConflict, currentConflictExists := c.conflictsByID.Get(conflictID)
		addedParent, addedParentExists := c.Conflicts(addedParentID)[addedParentID]
		removedParents := lo.Values(c.Conflicts(removedParentIDs...))

		if !currentConflictExists || !addedParentExists {
			return nil, nil, nil, false
		}

		return currentConflict, addedParent, removedParents, currentConflict.UpdateParents(addedParent, removedParents...)
	}()

	if updated {
		c.ConflictParentsUpdated.Trigger(currentConflict, addedParent, removedParents)
	}

	return updated
}

// LikedInstead returns the ConflictIDs of the Conflicts that are liked instead of the Conflicts.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) LikedInstead(conflictIDs ...ConflictID) map[ConflictID]*conflict.Conflict[ConflictID, ResourceID, VotePower] {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.pendingTasks.WaitIsZero()

	likedInstead := make(map[ConflictID]*conflict.Conflict[ConflictID, ResourceID, VotePower])
	for _, conflictID := range conflictIDs {
		if currentConflict, exists := c.conflictsByID.Get(conflictID); exists {
			if largestConflict := largestConflict(currentConflict.LikedInstead()); largestConflict != nil {
				likedInstead[largestConflict.ID] = largestConflict
			}
		}
	}

	return likedInstead
}

// Conflicts returns the Conflicts that are associated with the given ConflictIDs.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) Conflicts(ids ...ConflictID) map[ConflictID]*conflict.Conflict[ConflictID, ResourceID, VotePower] {
	conflicts := make(map[ConflictID]*conflict.Conflict[ConflictID, ResourceID, VotePower])
	for _, id := range ids {
		if existingConflict, exists := c.conflictsByID.Get(id); exists {
			conflicts[id] = existingConflict
		}
	}

	return conflicts
}

// ConflictSets returns the ConflictSets that are associated with the given ResourceIDs.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) ConflictSets(resourceIDs ...ResourceID) map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower] {
	conflictSets := make(map[ResourceID]*conflict.Set[ConflictID, ResourceID, VotePower])
	for _, resourceID := range resourceIDs {
		conflictSets[resourceID] = lo.Return1(c.conflictSetsByID.GetOrCreate(resourceID, func() *conflict.Set[ConflictID, ResourceID, VotePower] {
			return conflict.NewSet[ConflictID, ResourceID, VotePower](resourceID)
		}))
	}

	return conflictSets
}

// CastVotes applies the given votes to the ConflictDAG.
func (c *ConflictDAG[ConflictID, ResourceID, VotePower]) CastVotes(vote *vote.Vote[VotePower], conflictIDs ...ConflictID) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	supportedConflicts := advancedset.New[*conflict.Conflict[ConflictID, ResourceID, VotePower]]()
	revokedConflicts := advancedset.New[*conflict.Conflict[ConflictID, ResourceID, VotePower]]()

	revokedWalker := walker.New[*conflict.Conflict[ConflictID, ResourceID, VotePower]]()
	revokeConflict := func(revokedConflict *conflict.Conflict[ConflictID, ResourceID, VotePower]) error {
		if revokedConflicts.Add(revokedConflict) {
			if supportedConflicts.Has(revokedConflict) {
				return xerrors.Errorf("applied conflicting votes (%s is supported and revoked)", revokedConflict.ID)
			}

			revokedWalker.PushAll(revokedConflict.Children.Slice()...)
		}

		return nil
	}

	supportedWalker := walker.New[*conflict.Conflict[ConflictID, ResourceID, VotePower]]().PushAll(lo.Values(c.Conflicts(conflictIDs...))...)
	supportConflict := func(supportedConflict *conflict.Conflict[ConflictID, ResourceID, VotePower]) error {
		if supportedConflicts.Add(supportedConflict) {
			if err := supportedConflict.ConflictingConflicts.ForEach(func(revokedConflict *conflict.Conflict[ConflictID, ResourceID, VotePower]) error {
				if revokedConflict == supportedConflict {
					return nil
				}

				return revokeConflict(revokedConflict)
			}); err != nil {
				return xerrors.Errorf("failed to collect conflicting conflicts: %w", err)
			}

			supportedWalker.PushAll(supportedConflict.Parents.Slice()...)
		}

		return nil
	}

	for supportedWalker.HasNext() {
		if err := supportConflict(supportedWalker.Next()); err != nil {
			return xerrors.Errorf("failed to collect supported conflicts: %w", err)
		}
	}

	for revokedWalker.HasNext() {
		if err := revokeConflict(revokedWalker.Next()); err != nil {
			return xerrors.Errorf("failed to collect revoked conflicts: %w", err)
		}
	}

	for supportedConflict := supportedConflicts.Iterator(); supportedConflict.HasNext(); {
		supportedConflict.Next().ApplyVote(vote)
	}

	for revokedConflict := revokedConflicts.Iterator(); revokedConflict.HasNext(); {
		revokedConflict.Next().ApplyVote(vote.WithLiked(false))
	}

	return nil
}
