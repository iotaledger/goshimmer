package votes

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	markers2 "github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

type SequenceTracker[VotePowerType VotePower[VotePowerType]] struct {
	votes *memstorage.Storage[markers2.SequenceID, *memstorage.Storage[identity.ID, *LatestMarkerVotes[VotePowerType]]]

	sequenceCallback    func(id markers2.SequenceID) (sequence *markers2.Sequence, exists bool)
	validatorSet        *validator.Set
	cutoffIndexCallback func(sequenceID markers2.SequenceID) markers2.Index

	Events *SequenceTrackerEvents
}

func NewSequenceTracker[VotePowerType VotePower[VotePowerType]](validatorSet *validator.Set, sequenceCallback func(id markers2.SequenceID) (sequence *markers2.Sequence, exists bool), cutoffIndexCallback func(sequenceID markers2.SequenceID) markers2.Index) *SequenceTracker[VotePowerType] {
	return &SequenceTracker[VotePowerType]{
		votes:               memstorage.New[markers2.SequenceID, *memstorage.Storage[identity.ID, *LatestMarkerVotes[VotePowerType]]](),
		sequenceCallback:    sequenceCallback,
		validatorSet:        validatorSet,
		cutoffIndexCallback: cutoffIndexCallback,
		Events:              newSequenceTrackerEvents(),
	}
}

func (s *SequenceTracker[VotePowerType]) TrackVotes(pastMarkers *markers2.Markers, voterID identity.ID, power VotePowerType) {
	voter, exists := s.validatorSet.Get(voterID)
	if !exists {
		return
	}

	// Do not revisit markers that have already been visited. With the like reference there can be cycles in the sequence DAG
	// which results in endless walks.
	supportWalker := walker.New[markers2.Marker](false)

	pastMarkers.ForEach(func(sequenceID markers2.SequenceID, index markers2.Index) bool {
		supportWalker.Push(markers2.NewMarker(sequenceID, index))
		return true
	})

	for supportWalker.HasNext() {
		s.addVoteToMarker(supportWalker.Next(), voter, power, supportWalker)
	}
}

func (s *SequenceTracker[VotePowerType]) Voters(marker markers2.Marker) (voters *set.AdvancedSet[*validator.Validator]) {
	voters = set.NewAdvancedSet[*validator.Validator]()
	votes, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}

	votes.ForEach(func(identityID identity.ID, validatorVotes *LatestMarkerVotes[VotePowerType]) bool {
		_, voteExists := validatorVotes.Power(marker.Index())
		if !voteExists {
			return true
		}

		voter, validatorExists := s.validatorSet.Get(identityID)
		if validatorExists {
			voters.Add(voter)
		}
		return true
	})

	return
}

func (s *SequenceTracker[VotePowerType]) VotersWithPower(marker markers2.Marker) (voters map[identity.ID]VotePowerType) {
	voters = make(map[identity.ID]VotePowerType)

	votes, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}
	votes.ForEach(func(identityID identity.ID, validatorVotes *LatestMarkerVotes[VotePowerType]) bool {
		power, voteExists := validatorVotes.Power(marker.Index())
		if !voteExists {
			return true
		}

		voter, validatorExists := s.validatorSet.Get(identityID)
		if validatorExists {
			voters[voter.ID()] = power
		}
		return true
	})

	return
}

func (s *SequenceTracker[VotePowerType]) addVoteToMarker(marker markers2.Marker, voter *validator.Validator, power VotePowerType, walk *walker.Walker[markers2.Marker]) {
	// We don't add the voter and abort if the marker is already accepted/confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new conflict once a conflict arrives and we copy over the
	// voters of the marker to the conflict. Since the marker is already seen as confirmed it should not matter too much though.
	if s.cutoffIndexCallback(marker.SequenceID()) >= marker.Index() {
		return
	}

	sequenceStorage, _ := s.votes.RetrieveOrCreate(marker.SequenceID(), memstorage.New[identity.ID, *LatestMarkerVotes[VotePowerType]])
	latestMarkerVotes, _ := sequenceStorage.RetrieveOrCreate(voter.ID(), func() *LatestMarkerVotes[VotePowerType] {
		return NewLatestMarkerVotes[VotePowerType](voter)
	})

	stored, previousHighestIndex := latestMarkerVotes.Store(marker.Index(), power)
	if !stored {
		return
	}

	// Trigger events for all newly supported markers.
	for i := previousHighestIndex + 1; i <= marker.Index(); i++ {
		s.Events.VoterAdded.Trigger(&SequenceVoterEvent{
			Voter:  voter,
			Marker: markers2.NewMarker(marker.SequenceID(), i),
		})
	}

	// Walk the SequenceDAG to propagate votes to referenced sequences.
	sequence, exists := s.sequenceCallback(marker.SequenceID())
	if !exists {
		panic(fmt.Sprintf("sequence %d does not exist", marker.SequenceID()))
	}
	sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers2.SequenceID, index markers2.Index) bool {
		walk.Push(markers2.NewMarker(sequenceID, index))
		return true
	})
}
