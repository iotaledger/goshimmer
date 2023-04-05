package conflict

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

// SortedSet is a set of Conflicts that is sorted by their weight.
type SortedSet[ConflictID, ResourceID IDType] struct {
	// owner is the Conflict that owns this SortedSet.
	owner *Conflict[ConflictID, ResourceID]

	// members is a map of ConflictIDs to their corresponding sortedSetMember.
	members *shrinkingmap.ShrinkingMap[ConflictID, *sortedSetMember[ConflictID, ResourceID]]

	// heaviestMember is the heaviest member of the SortedSet.
	heaviestMember *sortedSetMember[ConflictID, ResourceID]

	// heaviestPreferredMember is the heaviest preferred member of the SortedSet.
	heaviestPreferredMember *sortedSetMember[ConflictID, ResourceID]

	// pendingWeightUpdates is a collection of Conflicts that have a pending weight update.
	pendingWeightUpdates *shrinkingmap.ShrinkingMap[ConflictID, *sortedSetMember[ConflictID, ResourceID]]

	// pendingWeightUpdatesSignal is a signal that is used to notify the fixMemberPositionWorker about pending weight
	// updates.
	pendingWeightUpdatesSignal *sync.Cond

	// pendingWeightUpdatesMutex is a mutex that is used to synchronize access to the pendingWeightUpdates.
	pendingWeightUpdatesMutex sync.RWMutex

	// pendingPreferredInsteadUpdates is a collection of Conflicts that have a pending preferred instead update.
	pendingPreferredInsteadUpdates *shrinkingmap.ShrinkingMap[ConflictID, *sortedSetMember[ConflictID, ResourceID]]

	// pendingPreferredInsteadSignal is a signal that is used to notify the fixPreferredInsteadWorker about pending
	// preferred instead updates.
	pendingPreferredInsteadSignal *sync.Cond

	// pendingPreferredInsteadMutex is a mutex that is used to synchronize access to the pendingPreferredInsteadUpdates.
	pendingPreferredInsteadMutex sync.RWMutex

	// pendingUpdatesCounter is a counter that keeps track of the number of pending weight updates.
	pendingUpdatesCounter *syncutils.Counter

	// isShutdown is used to signal that the SortedSet is shutting down.
	isShutdown atomic.Bool

	// mutex is used to synchronize access to the SortedSet.
	mutex sync.RWMutex
}

// NewSortedSet creates a new SortedSet that is owned by the given Conflict.
func NewSortedSet[ConflictID, ResourceID IDType](owner *Conflict[ConflictID, ResourceID], pendingUpdatesCounter *syncutils.Counter) *SortedSet[ConflictID, ResourceID] {
	s := &SortedSet[ConflictID, ResourceID]{
		owner:                          owner,
		members:                        shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
		pendingWeightUpdates:           shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
		pendingUpdatesCounter:          pendingUpdatesCounter,
		pendingPreferredInsteadUpdates: shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
	}
	s.pendingWeightUpdatesSignal = sync.NewCond(&s.pendingWeightUpdatesMutex)
	s.pendingPreferredInsteadSignal = sync.NewCond(&s.pendingPreferredInsteadMutex)

	newMember := newSortedSetMember[ConflictID, ResourceID](s, owner)
	s.members.Set(owner.id, newMember)

	s.heaviestMember = newMember
	s.heaviestPreferredMember = newMember

	// TODO: move to WorkerPool so we are consistent with the rest of the codebase
	go s.fixMemberPositionWorker()
	go s.fixHeaviestPreferredMemberWorker()

	return s
}

// Add adds the given Conflict to the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newMember, isNew := s.members.GetOrCreate(conflict.id, func() *sortedSetMember[ConflictID, ResourceID] {
		return newSortedSetMember[ConflictID, ResourceID](s, conflict)
	})
	if !isNew {
		return false
	}

	for currentMember := s.heaviestMember; ; currentMember = currentMember.lighterMember {
		comparison := newMember.Compare(currentMember)
		if comparison == weight.Equal {
			panic("different Conflicts should never have the same weight")
		}

		if comparison == weight.Heavier {
			if currentMember.heavierMember != nil {
				currentMember.heavierMember.lighterMember = newMember
			}

			newMember.lighterMember = currentMember
			newMember.heavierMember = currentMember.heavierMember
			currentMember.heavierMember = newMember

			if currentMember == s.heaviestMember {
				s.heaviestMember = newMember
			}

			break
		}

		if currentMember.lighterMember == nil {
			currentMember.lighterMember = newMember
			newMember.heavierMember = currentMember

			break
		}
	}

	if newMember.IsPreferred() && newMember.Compare(s.heaviestPreferredMember) == weight.Heavier {
		s.heaviestPreferredMember = newMember

		s.owner.setPreferredInstead(conflict)
	}

	return true
}

// ForEach iterates over all Conflicts of the SortedSet and calls the given callback for each of them.
func (s *SortedSet[ConflictID, ResourceID]) ForEach(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for currentMember := s.heaviestMember; currentMember != nil; currentMember = currentMember.lighterMember {
		if err := callback(currentMember.Conflict); err != nil {
			return err
		}
	}

	return nil
}

// String returns a human-readable representation of the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) String() string {
	structBuilder := stringify.NewStructBuilder("SortedSet",
		stringify.NewStructField("owner", s.owner.ID()),
		stringify.NewStructField("heaviestMember", s.heaviestMember.ID()),
		stringify.NewStructField("heaviestPreferredMember", s.heaviestPreferredMember.ID()),
	)

	_ = s.ForEach(func(conflict *Conflict[ConflictID, ResourceID]) error {
		structBuilder.AddField(stringify.NewStructField(conflict.id.String(), conflict))
		return nil
	})

	return structBuilder.String()
}

// notifyPendingWeightUpdate notifies the SortedSet about a pending weight update of the given member.
func (s *SortedSet[ConflictID, ResourceID]) notifyPendingWeightUpdate(member *sortedSetMember[ConflictID, ResourceID]) {
	s.pendingWeightUpdatesMutex.Lock()
	defer s.pendingWeightUpdatesMutex.Unlock()

	if _, exists := s.pendingWeightUpdates.Get(member.id); !exists {
		s.pendingUpdatesCounter.Increase()
		s.pendingWeightUpdates.Set(member.id, member)
		s.pendingWeightUpdatesSignal.Signal()
	}
}

// fixMemberPositionWorker is a worker that fixes the position of sortedSetMembers that need to be updated.
func (s *SortedSet[ConflictID, ResourceID]) fixMemberPositionWorker() {
	for member := s.nextPendingWeightUpdate(); member != nil; member = s.nextPendingWeightUpdate() {
		s.applyWeightUpdate(member)

		s.pendingUpdatesCounter.Decrease()
	}
}

// nextPendingWeightUpdate returns the next member that needs to be updated (or nil if the shutdown flag is set).
func (s *SortedSet[ConflictID, ResourceID]) nextPendingWeightUpdate() *sortedSetMember[ConflictID, ResourceID] {
	s.pendingWeightUpdatesMutex.Lock()
	defer s.pendingWeightUpdatesMutex.Unlock()

	for !s.isShutdown.Load() && s.pendingWeightUpdates.Size() == 0 {
		s.pendingWeightUpdatesSignal.Wait()
	}

	if !s.isShutdown.Load() {
		if _, member, exists := s.pendingWeightUpdates.Pop(); exists {
			return member
		}
	}

	return nil
}

// applyWeightUpdate applies the weight update of the given member.
func (s *SortedSet[ConflictID, ResourceID]) applyWeightUpdate(member *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if member.weightUpdateApplied() {
		s.fixMemberPosition(member)
	}
}

// fixMemberPosition fixes the position of the given member in the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) fixMemberPosition(member *sortedSetMember[ConflictID, ResourceID]) {
	preferredConflict := member.PreferredInstead()
	memberIsPreferred := member.IsPreferred()

	// the member needs to be moved up in the list
	for currentMember := member.heavierMember; currentMember != nil && currentMember.Compare(member) == weight.Lighter; currentMember = member.heavierMember {
		s.swapNeighbors(member, currentMember)

		if currentMember == s.heaviestPreferredMember && (preferredConflict == currentMember.Conflict || memberIsPreferred || member.Conflict == s.owner) {
			s.heaviestPreferredMember = member
			s.owner.setPreferredInstead(member.Conflict)
		}
	}

	// the member needs to be moved down in the list
	for currentMember := member.lighterMember; currentMember != nil && currentMember.Compare(member) == weight.Heavier; currentMember = member.lighterMember {
		s.swapNeighbors(currentMember, member)

		if member == s.heaviestPreferredMember && (currentMember.IsPreferred() || currentMember.PreferredInstead() == member.Conflict || currentMember.Conflict == s.owner) {
			s.heaviestPreferredMember = currentMember
			s.owner.setPreferredInstead(currentMember.Conflict)
		}
	}
}

// notifyPreferredInsteadUpdate notifies the SortedSet about a member that changed its preferred instead flag.
func (s *SortedSet[ConflictID, ResourceID]) notifyPendingPreferredInsteadUpdate(member *sortedSetMember[ConflictID, ResourceID]) {
	s.pendingPreferredInsteadMutex.Lock()
	defer s.pendingPreferredInsteadMutex.Unlock()

	if _, exists := s.pendingPreferredInsteadUpdates.Get(member.id); !exists {
		s.pendingUpdatesCounter.Increase()
		s.pendingPreferredInsteadUpdates.Set(member.id, member)
		s.pendingPreferredInsteadSignal.Signal()
	}
}

// fixMemberPositionWorker is a worker that fixes the position of sortedSetMembers that need to be updated.
func (s *SortedSet[ConflictID, ResourceID]) fixHeaviestPreferredMemberWorker() {
	for member := s.nextPendingPreferredMemberUpdate(); member != nil; member = s.nextPendingPreferredMemberUpdate() {
		s.applyPreferredInsteadUpdate(member)

		s.pendingUpdatesCounter.Decrease()
	}
}

// nextPendingWeightUpdate returns the next member that needs to be updated (or nil if the shutdown flag is set).
func (s *SortedSet[ConflictID, ResourceID]) nextPendingPreferredMemberUpdate() *sortedSetMember[ConflictID, ResourceID] {
	s.pendingPreferredInsteadMutex.Lock()
	defer s.pendingPreferredInsteadMutex.Unlock()

	for !s.isShutdown.Load() && s.pendingPreferredInsteadUpdates.Size() == 0 {
		s.pendingPreferredInsteadSignal.Wait()
	}

	if !s.isShutdown.Load() {
		if _, member, exists := s.pendingPreferredInsteadUpdates.Pop(); exists {
			return member
		}
	}

	return nil
}

// applyPreferredInsteadUpdate applies the preferred instead update of the given member.
func (s *SortedSet[ConflictID, ResourceID]) applyPreferredInsteadUpdate(member *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if member.preferredInsteadUpdateApplied() {
		s.fixHeaviestPreferredMember(member)
	}
}

// fixHeaviestPreferredMember fixes the heaviest preferred member of the SortedSet after updating the given member.
func (s *SortedSet[ConflictID, ResourceID]) fixHeaviestPreferredMember(member *sortedSetMember[ConflictID, ResourceID]) {
	if member.IsPreferred() {
		if member.Compare(s.heaviestPreferredMember) == weight.Heavier {
			s.heaviestPreferredMember = member
			s.owner.setPreferredInstead(member.Conflict)
		}

		return
	}

	if s.heaviestPreferredMember == member {
		for currentMember := member; ; currentMember = currentMember.lighterMember {
			if currentMember.Conflict == s.owner || currentMember.IsPreferred() || currentMember.PreferredInstead() == member.Conflict {
				s.heaviestPreferredMember = currentMember
				s.owner.setPreferredInstead(currentMember.Conflict)

				return
			}
		}
	}
}

// swapNeighbors swaps the given members in the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) swapNeighbors(heavierMember, lighterMember *sortedSetMember[ConflictID, ResourceID]) {
	if heavierMember.lighterMember != nil {
		heavierMember.lighterMember.heavierMember = lighterMember
	}
	if lighterMember.heavierMember != nil {
		lighterMember.heavierMember.lighterMember = heavierMember
	}

	lighterMember.lighterMember = heavierMember.lighterMember
	heavierMember.heavierMember = lighterMember.heavierMember
	lighterMember.heavierMember = heavierMember
	heavierMember.lighterMember = lighterMember

	if s.heaviestMember == lighterMember {
		s.heaviestMember = heavierMember
	}
}
