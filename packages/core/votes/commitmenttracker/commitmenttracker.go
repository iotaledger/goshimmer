package commitmenttracker

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

type CommitmentTracker struct {
	votes      *memstorage.Storage[identity.ID, *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]]
	pruningMap *memstorage.Storage[epoch.Index, *set.AdvancedSet[identity.ID]]

	validatorSet        *validator.Set
	cutoffIndexCallback func() epoch.Index
}

func NewCommitmentTracker(validatorSet *validator.Set, cutoffIndexCallback func() epoch.Index) *CommitmentTracker {
	return &CommitmentTracker{
		votes:      memstorage.New[identity.ID, *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]](),
		pruningMap: memstorage.New[epoch.Index, *set.AdvancedSet[identity.ID]](),

		validatorSet:        validatorSet,
		cutoffIndexCallback: cutoffIndexCallback,
	}
}

func (s *CommitmentTracker) TrackVotes(epochIndex epoch.Index, voterID identity.ID, power CommitmentVotePower) {
	voter, exists := s.validatorSet.Get(voterID)
	if !exists {
		return
	}

	epochVoters, _ := s.pruningMap.RetrieveOrCreate(e, func() *set.AdvancedSet[identity.ID] {
		return set.NewAdvancedSet[identity.ID]()
	})
	if epochVoters.Has(voterID) {
		return
	}

	votersVotes, _ := s.votes.RetrieveOrCreate(voterID, func() *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower] {
		return latestvotes.NewLatestVotes[epoch.Index, CommitmentVotePower](voter)
	})

	stored, previousHighestIndex := votersVotes.Store(epochIndex, power)

	if !stored {
		return
	}

	for i := previousHighestIndex; i >= epochIndex
}

func (s *CommitmentTracker) Voters(marker markers.Marker) (voters *validator.Set) {
	voters = validator.NewSet()
	votesObj, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}

	votesObj.ForEach(func(identityID identity.ID, validatorVotes *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]) bool {
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

func (s *CommitmentTracker) VotersWithPower(marker markers.Marker) (voters map[identity.ID]CommitmentVotePower) {
	voters = make(map[identity.ID]CommitmentVotePower)

	votesObj, exists := s.votes.Get(marker.SequenceID())
	if !exists {
		return
	}
	votesObj.ForEach(func(identityID identity.ID, validatorVotes *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]) bool {
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

func (s *CommitmentTracker) addVoteToMarker(marker markers.Marker, voter *validator.Validator, power CommitmentVotePower, walk *walker.Walker[markers.Marker]) {
	// We don't add the voter and abort if the marker is already accepted/confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new conflict once a conflict arrives, and we copy over the
	// voters of the marker to the conflict. Since the marker is already seen as confirmed it should not matter too much though.
	if s.cutoffIndexCallback(marker.SequenceID()) >= marker.Index() {
		return
	}

	sequenceStorage, _ := s.votes.RetrieveOrCreate(marker.SequenceID(), memstorage.New[identity.ID, *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]])
	latestMarkerVotes, _ := sequenceStorage.RetrieveOrCreate(voter.ID(), func() *latestvotes.LatestVotes[CommitmentVotePower] {
		return latestvotes.NewLatestVotes[CommitmentVotePower](voter)
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

// region MockedVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type CommitmentVotePower struct {
	VotePower epoch.Index
}

func (p CommitmentVotePower) Compare(other CommitmentVotePower) int {
	if p.VotePower-other.VotePower < 0 {
		return -1
	} else if p.VotePower-other.VotePower > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
