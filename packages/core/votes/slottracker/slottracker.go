package slottracker

import (
	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
)

type SlotTracker struct {
	Events *Events

	votesPerIdentity *shrinkingmap.ShrinkingMap[identity.ID, *latestvotes.LatestVotes[slot.Index, SlotVotePower]]
	votersPerSlot    *shrinkingmap.ShrinkingMap[slot.Index, *advancedset.AdvancedSet[identity.ID]]

	cutoffIndexCallback func() slot.Index
}

func NewSlotTracker(cutoffIndexCallback func() slot.Index) *SlotTracker {
	return &SlotTracker{
		votesPerIdentity: shrinkingmap.New[identity.ID, *latestvotes.LatestVotes[slot.Index, SlotVotePower]](),
		votersPerSlot:    shrinkingmap.New[slot.Index, *advancedset.AdvancedSet[identity.ID]](),

		cutoffIndexCallback: cutoffIndexCallback,
		Events:              NewEvents(),
	}
}

func (s *SlotTracker) slotVoters(slotIndex slot.Index) *advancedset.AdvancedSet[identity.ID] {
	slotVoters, _ := s.votersPerSlot.GetOrCreate(slotIndex, func() *advancedset.AdvancedSet[identity.ID] {
		return advancedset.New[identity.ID]()
	})
	return slotVoters
}

func (s *SlotTracker) TrackVotes(slotIndex slot.Index, voterID identity.ID, power SlotVotePower) {
	slotVoters := s.slotVoters(slotIndex)
	if slotVoters.Has(voterID) {
		// We already tracked the voter for this slot, so no need to update anything
		return
	}

	votersVotes, _ := s.votesPerIdentity.GetOrCreate(voterID, func() *latestvotes.LatestVotes[slot.Index, SlotVotePower] {
		return latestvotes.NewLatestVotes[slot.Index, SlotVotePower](voterID)
	})

	updated, previousHighestIndex := votersVotes.Store(slotIndex, power)
	if !updated || previousHighestIndex >= slotIndex {
		return
	}

	for i := lo.Max(s.cutoffIndexCallback(), previousHighestIndex) + 1; i <= slotIndex; i++ {
		s.slotVoters(i).Add(voterID)
	}

	s.Events.VotersUpdated.Trigger(&VoterUpdatedEvent{
		Voter:               voterID,
		NewLatestSlotIndex:  slotIndex,
		PrevLatestSlotIndex: previousHighestIndex,
	})
}

func (s *SlotTracker) Voters(slotIndex slot.Index) *advancedset.AdvancedSet[identity.ID] {
	voters := advancedset.New[identity.ID]()

	slotVoters, exists := s.votersPerSlot.Get(slotIndex)
	if !exists {
		return voters
	}

	_ = slotVoters.ForEach(func(identityID identity.ID) error {
		voters.Add(identityID)
		return nil
	})

	return voters
}

func (s *SlotTracker) EvictSlot(indexToEvict slot.Index) {
	identities, exists := s.votersPerSlot.Get(indexToEvict)
	if !exists {
		return
	}

	var identitiesToPrune []identity.ID
	_ = identities.ForEach(func(identity identity.ID) error {
		votesForIdentity, has := s.votesPerIdentity.Get(identity)
		if !has {
			return nil
		}
		power, hasPower := votesForIdentity.Power(indexToEvict)
		if !hasPower {
			return nil
		}
		if power.Index <= indexToEvict {
			identitiesToPrune = append(identitiesToPrune, identity)
		}
		return nil
	})
	for _, identity := range identitiesToPrune {
		s.votesPerIdentity.Delete(identity)
	}

	s.votersPerSlot.Delete(indexToEvict)
}

// region SlotVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type SlotVotePower struct {
	Index slot.Index
}

func (p SlotVotePower) Compare(other SlotVotePower) int {
	if p.Index-other.Index < 0 {
		return -1
	} else if p.Index-other.Index > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
