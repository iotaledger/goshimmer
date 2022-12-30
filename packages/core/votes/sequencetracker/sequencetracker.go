package sequencetracker

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

type SequenceTracker[VotePowerType constraints.Comparable[VotePowerType]] struct {
	Events *Events

	votes *memstorage.Storage[markers.SequenceID, *memstorage.Storage[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]]]

	sequenceCallback    func(id markers.SequenceID) (sequence *markers.Sequence, exists bool)
	validators          *sybilprotection.WeightedSet
	cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index
}

func NewSequenceTracker[VotePowerType constraints.Comparable[VotePowerType]](validators *sybilprotection.WeightedSet, sequenceCallback func(id markers.SequenceID) (sequence *markers.Sequence, exists bool), cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index) *SequenceTracker[VotePowerType] {
	return &SequenceTracker[VotePowerType]{
		votes:               memstorage.New[markers.SequenceID, *memstorage.Storage[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]]](),
		sequenceCallback:    sequenceCallback,
		validators:          validators,
		cutoffIndexCallback: cutoffIndexCallback,
		Events:              NewEvents(),
	}
}

func (s *SequenceTracker[VotePowerType]) TrackVotes(pastMarkers *markers.Markers, voterID identity.ID, power VotePowerType) {
	// Do not revisit markers that have already been visited. With the like reference there can be cycles in the sequence DAG
	// which results in endless walks.
	supportWalker := walker.New[markers.Marker](false)

	pastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		supportWalker.Push(markers.NewMarker(sequenceID, index))
		return true
	})

	for supportWalker.HasNext() {
		s.addVoteToMarker(supportWalker.Next(), voterID, power, supportWalker)
	}
}

func (s *SequenceTracker[VotePowerType]) Voters(marker markers.Marker) (voters *set.AdvancedSet[identity.ID]) {
	voters = set.NewAdvancedSet[identity.ID]()

	votesObj, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}
	loopCounter := 0
	outerNow := time.Now()

	votesObj.ForEach(func(identityID identity.ID, validatorVotes *latestvotes.LatestVotes[markers.Index, VotePowerType]) bool {
		loopCounter++
		now := time.Now()
		_, voteExists := validatorVotes.Power(marker.Index())
		if !voteExists {
			if duration := time.Since(now); duration > 100*time.Millisecond {
				fmt.Println("Adding voter identity took more than one second:", duration)
			}
			return true
		}
		voters.Add(identityID)
		if duration := time.Since(now); duration > 100*time.Millisecond {
			fmt.Println("Adding voter identity took more than one second:", duration)
		}
		return true
	})

	if duration := time.Since(outerNow); loopCounter > 10 || duration > 100*time.Millisecond {
		fmt.Println("Loop counter:", loopCounter, "loop duration:", duration, "len(votesObj)", votesObj.Size(), "votesObj (should contain 9 entries):", lo.Keys(votesObj.AsMap()))
	}
	return
}

func (s *SequenceTracker[VotePowerType]) VotersWithPower(marker markers.Marker) (voters map[identity.ID]VotePowerType) {
	voters = make(map[identity.ID]VotePowerType)

	votesObj, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}
	votesObj.ForEach(func(identityID identity.ID, validatorVotes *latestvotes.LatestVotes[markers.Index, VotePowerType]) bool {
		power, voteExists := validatorVotes.Power(marker.Index())
		if !voteExists {
			return true
		}

		if s.validators.Has(identityID) {
			voters[identityID] = power
		}
		return true
	})

	return
}

func (s *SequenceTracker[VotePowerType]) addVoteToMarker(marker markers.Marker, voter identity.ID, power VotePowerType, walk *walker.Walker[markers.Marker]) {
	// We don't add the voter and abort if the marker is already accepted/confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new conflict once a conflict arrives, and we copy over the
	// voters of the marker to the conflict. Since the marker is already seen as confirmed it should not matter too much though.
	if marker.Index() < s.cutoffIndexCallback(marker.SequenceID()) {
		return
	}

	sequenceStorage, _ := s.votes.RetrieveOrCreate(marker.SequenceID(), memstorage.New[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]])
	latestMarkerVotes, _ := sequenceStorage.RetrieveOrCreate(voter, func() *latestvotes.LatestVotes[markers.Index, VotePowerType] {
		return latestvotes.NewLatestVotes[markers.Index, VotePowerType](voter)
	})

	stored, previousHighestIndex := latestMarkerVotes.Store(marker.Index(), power)
	if !stored {
		return
	}

	if previousHighestIndex == 0 {
		sequence, _ := s.sequenceCallback(marker.SequenceID())
		previousHighestIndex = sequence.LowestIndex()
	}

	s.Events.VotersUpdated.Trigger(&VoterUpdatedEvent{
		Voter:                 voter,
		NewMaxSupportedIndex:  marker.Index(),
		PrevMaxSupportedIndex: previousHighestIndex,
		SequenceID:            marker.SequenceID(),
	}, "sequence voter added")

	// Walk the SequenceDAG to propagate votes to referenced sequences.
	sequence, exists := s.sequenceCallback(marker.SequenceID())
	if !exists {
		panic(fmt.Sprintf("sequence %d does not exist", marker.SequenceID()))
	}
	sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		walk.Push(markers.NewMarker(sequenceID, index))
		return true
	})
}

func (s *SequenceTracker[VotePowerType]) EvictSequence(sequenceID markers.SequenceID) {
	s.votes.Delete(sequenceID)
}
