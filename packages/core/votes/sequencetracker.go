package votes

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type SequenceTracker[VotePowerType VotePower[VotePowerType]] struct {
	votes *memstorage.Storage[markers.SequenceID, *memstorage.Storage[identity.ID, *LatestMarkerVotes[VotePowerType]]]

	sequenceManager     *markers.SequenceManager
	validatorSet        *validator.Set
	cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index

	Events *SequenceTrackerEvents
}

func NewSequenceTracker[VotePowerType VotePower[VotePowerType]](sequenceManager *markers.SequenceManager, validatorSet *validator.Set, cutoffIndexCallback func(sequenceID markers.SequenceID) markers.Index) *SequenceTracker[VotePowerType] {
	return &SequenceTracker[VotePowerType]{
		votes:               memstorage.New[markers.SequenceID, *memstorage.Storage[identity.ID, *LatestMarkerVotes[VotePowerType]]](),
		sequenceManager:     sequenceManager,
		validatorSet:        validatorSet,
		cutoffIndexCallback: cutoffIndexCallback,
		Events:              newSequenceTrackerEvents(),
	}
}

func (s *SequenceTracker[VotePowerType]) TrackVotes(pastMarkers *markers.Markers, voterID identity.ID, power VotePowerType) {
	voter, exists := s.validatorSet.Get(voterID)
	if !exists {
		return
	}

	// Do not revisit markers that have already been visited. With the like reference there can be cycles in the sequence DAG
	// which results in endless walks.
	supportWalker := walker.New[markers.Marker](false)

	pastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		supportWalker.Push(markers.NewMarker(sequenceID, index))
		return true
	})

	for supportWalker.HasNext() {
		s.addVoteToMarker(supportWalker.Next(), voter, power, supportWalker)
	}
}

func (s *SequenceTracker[VotePowerType]) addVoteToMarker(marker markers.Marker, voter *validator.Validator, power VotePowerType, walk *walker.Walker[markers.Marker]) {
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
	for i := previousHighestIndex; i < marker.Index(); i++ {
		s.Events.VoterAdded.Trigger(&SequenceVoterEvent{
			Voter:  voter,
			Marker: marker,
		})
	}

	// Walk the SequenceDAG to propagate votes to referenced sequences.
	sequence, exists := s.sequenceManager.Sequence(marker.SequenceID())
	if !exists {
		panic(fmt.Sprintf("sequence %d does not exist", marker.SequenceID()))
	}
	sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		walk.Push(markers.NewMarker(sequenceID, index))
		return true
	})
}
