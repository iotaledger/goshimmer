package sequencetracker

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
)

type SequenceTracker[VotePowerType constraints.Comparable[VotePowerType]] struct {
	Events *Events

	votes *shrinkingmap.ShrinkingMap[markers.SequenceID, *shrinkingmap.ShrinkingMap[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]]]

	sequenceCallback    func(id markers.SequenceID) (sequence *markers.Sequence, exists bool)
	validators          *sybilprotection.WeightedSet
	cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index
}

func NewSequenceTracker[VotePowerType constraints.Comparable[VotePowerType]](validators *sybilprotection.WeightedSet, sequenceCallback func(id markers.SequenceID) (sequence *markers.Sequence, exists bool), cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index) *SequenceTracker[VotePowerType] {
	return &SequenceTracker[VotePowerType]{
		votes:               shrinkingmap.New[markers.SequenceID, *shrinkingmap.ShrinkingMap[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]]](),
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

func (s *SequenceTracker[VotePowerType]) Voters(marker markers.Marker) (voters *advancedset.AdvancedSet[identity.ID]) {
	voters = advancedset.New[identity.ID]()

	votesObj, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}

	votesObj.ForEach(func(identityID identity.ID, validatorVotes *latestvotes.LatestVotes[markers.Index, VotePowerType]) bool {
		_, voteExists := validatorVotes.Power(marker.Index())
		if !voteExists {
			return true
		}

		voters.Add(identityID)

		return true
	})

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

	sequenceStorage, _ := s.votes.GetOrCreate(marker.SequenceID(), func() *shrinkingmap.ShrinkingMap[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]] {
		return shrinkingmap.New[identity.ID, *latestvotes.LatestVotes[markers.Index, VotePowerType]]()
	})
	latestMarkerVotes, _ := sequenceStorage.GetOrCreate(voter, func() *latestvotes.LatestVotes[markers.Index, VotePowerType] {
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
	})

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
