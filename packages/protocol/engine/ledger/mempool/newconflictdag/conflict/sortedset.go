package conflict

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

// SortedSet is a set of Conflicts that is sorted by their weight.
type SortedSet[ConflictID, ResourceID IDType] struct {
	// HeaviestPreferredMemberUpdated is triggered when the heaviest preferred member of the SortedSet changes.
	HeaviestPreferredMemberUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

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

	// pendingWeightUpdatesCounter is a counter that keeps track of the number of pending weight updates.
	pendingWeightUpdatesCounter *syncutils.Counter

	// pendingWeightUpdatesSignal is a signal that is used to notify the fixMemberPositionWorker about pending weight updates.
	pendingWeightUpdatesSignal *sync.Cond

	// pendingWeightUpdatesMutex is a mutex that is used to synchronize access to the pendingWeightUpdates.
	pendingWeightUpdatesMutex sync.RWMutex

	// isShutdown is used to signal that the SortedSet is shutting down.
	isShutdown atomic.Bool

	// mutex is used to synchronize access to the SortedSet.
	mutex sync.RWMutex
}

// NewSortedSet creates a new SortedSet that is owned by the given Conflict.
func NewSortedSet[ConflictID, ResourceID IDType](owner *Conflict[ConflictID, ResourceID]) *SortedSet[ConflictID, ResourceID] {
	s := &SortedSet[ConflictID, ResourceID]{
		HeaviestPreferredMemberUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		owner:                          owner,
		members:                        shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
		pendingWeightUpdates:           shrinkingmap.New[ConflictID, *sortedSetMember[ConflictID, ResourceID]](),
		pendingWeightUpdatesCounter:    syncutils.NewCounter(),
	}
	s.pendingWeightUpdatesSignal = sync.NewCond(&s.pendingWeightUpdatesMutex)

	s.Add(owner)

	// TODO: move to WorkerPool so we are consistent with the rest of the codebase
	go s.fixMemberPositionWorker()

	return s
}

// Add adds the given Conflict to the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) Add(conflict *Conflict[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newMember, isNew := s.members.GetOrCreate(conflict.id, func() *sortedSetMember[ConflictID, ResourceID] {
		return newSortedSetMember[ConflictID, ResourceID](s, conflict)
	})

	if !isNew {
		return
	}

	if conflict == s.owner {
		s.heaviestMember = newMember
		s.heaviestPreferredMember = newMember

		return
	}

	if conflict.IsPreferred() && newMember.Compare(s.heaviestPreferredMember) == weight.Heavier {
		s.heaviestPreferredMember = newMember

		s.HeaviestPreferredMemberUpdated.Trigger(conflict)
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

// HeaviestConflict returns the heaviest Conflict of the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) HeaviestConflict() *Conflict[ConflictID, ResourceID] {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.heaviestMember == nil {
		return nil
	}

	return s.heaviestMember.Conflict
}

// HeaviestPreferredConflict returns the heaviest preferred Conflict of the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) HeaviestPreferredConflict() *Conflict[ConflictID, ResourceID] {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.heaviestPreferredMember == nil {
		return nil
	}

	return s.heaviestPreferredMember.Conflict
}

// WaitConsistent waits until the SortedSet is consistent.
func (s *SortedSet[ConflictID, ResourceID]) WaitConsistent() {
	s.pendingWeightUpdatesCounter.WaitIsZero()

	// TODO: Wait until the last update has been applied (the counter becomes zero before "being done")
}

// String returns a human-readable representation of the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) String() string {
	structBuilder := stringify.NewStructBuilder("SortedSet")
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
		s.pendingWeightUpdatesCounter.Increase()
		s.pendingWeightUpdates.Set(member.id, member)
		s.pendingWeightUpdatesSignal.Signal()
	}
}

// notifyPreferredInsteadUpdate notifies the SortedSet about a member that changed its preferred instead flag.
func (s *SortedSet[ConflictID, ResourceID]) notifyPreferredInsteadUpdate(member *sortedSetMember[ConflictID, ResourceID], preferred bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if preferred {
		if member.Compare(s.heaviestPreferredMember) == weight.Heavier {
			s.heaviestPreferredMember = member
			s.HeaviestPreferredMemberUpdated.Trigger(member.Conflict)
		}

		return
	}

	if s.heaviestPreferredMember != member {
		return
	}

	currentMember := member.lighterMember
	for currentMember.Conflict != s.owner && !currentMember.IsPreferred() && currentMember.PreferredInstead() != member.Conflict {
		currentMember = currentMember.lighterMember
	}

	s.heaviestPreferredMember = currentMember
	s.HeaviestPreferredMemberUpdated.Trigger(currentMember.Conflict)
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
			s.pendingWeightUpdatesCounter.Decrease()

			return member
		}
	}

	return nil
}

// fixMemberPositionWorker is a worker that fixes the position of sortedSetMembers that need to be updated.
func (s *SortedSet[ConflictID, ResourceID]) fixMemberPositionWorker() {
	for member := s.nextPendingWeightUpdate(); member != nil; member = s.nextPendingWeightUpdate() {
		if member.weightUpdateApplied() {
			s.fixMemberPosition(member)
		}
	}
}

// fixMemberPosition fixes the position of the given member in the SortedSet.
func (s *SortedSet[ConflictID, ResourceID]) fixMemberPosition(member *sortedSetMember[ConflictID, ResourceID]) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// the member needs to be moved up in the list
	memberIsPreferred := (member.Conflict == s.owner && member == s.heaviestPreferredMember) || member.IsPreferred()
	for currentMember := member.heavierMember; currentMember != nil && currentMember.Compare(member) == weight.Lighter; currentMember = member.heavierMember {
		s.swapNeighbors(member, currentMember)

		if memberIsPreferred && currentMember == s.heaviestPreferredMember {
			s.heaviestPreferredMember = member
			s.HeaviestPreferredMemberUpdated.Trigger(member.Conflict)
		}
	}

	// the member needs to be moved down in the list
	memberIsHeaviestPreferred := member == s.heaviestPreferredMember
	for currentMember := member.lighterMember; currentMember != nil && currentMember.Compare(member) == weight.Heavier; currentMember = member.lighterMember {
		s.swapNeighbors(currentMember, member)

		if memberIsHeaviestPreferred && currentMember.IsPreferred() {
			s.heaviestPreferredMember = currentMember
			s.HeaviestPreferredMemberUpdated.Trigger(currentMember.Conflict)

			memberIsHeaviestPreferred = false
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
