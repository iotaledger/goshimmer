package commitmenttracker

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
)

type CommitmentTracker struct {
	Events *Events

	votesPerIdentity *memstorage.Storage[identity.ID, *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]]
	votersPerEpoch   *memstorage.Storage[epoch.Index, *set.AdvancedSet[identity.ID]]

	validatorSet        *validator.Set
	cutoffIndexCallback func() epoch.Index
}

func NewCommitmentTracker(validatorSet *validator.Set, cutoffIndexCallback func() epoch.Index) *CommitmentTracker {
	return &CommitmentTracker{
		votesPerIdentity: memstorage.New[identity.ID, *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower]](),
		votersPerEpoch:   memstorage.New[epoch.Index, *set.AdvancedSet[identity.ID]](),

		validatorSet:        validatorSet,
		cutoffIndexCallback: cutoffIndexCallback,
		Events:              NewEvents(),
	}
}

func (c *CommitmentTracker) epochVoters(epochIndex epoch.Index) *set.AdvancedSet[identity.ID] {
	epochVoters, _ := c.votersPerEpoch.RetrieveOrCreate(epochIndex, func() *set.AdvancedSet[identity.ID] {
		return set.NewAdvancedSet[identity.ID]()
	})
	return epochVoters
}

func (c *CommitmentTracker) TrackVotes(epochIndex epoch.Index, voterID identity.ID, power CommitmentVotePower) {
	voter, exists := c.validatorSet.Get(voterID)
	if !exists {
		return
	}

	epochVoters := c.epochVoters(epochIndex)
	if epochVoters.Has(voterID) {
		// We already tracked the voter for this epoch, so no need to update anything
		return
	}

	votersVotes, _ := c.votesPerIdentity.RetrieveOrCreate(voterID, func() *latestvotes.LatestVotes[epoch.Index, CommitmentVotePower] {
		return latestvotes.NewLatestVotes[epoch.Index, CommitmentVotePower](voter)
	})

	_, previousHighestIndex := votersVotes.Store(epochIndex, power)
	for i := lo.Max(c.cutoffIndexCallback(), previousHighestIndex+1); i <= epochIndex; i++ {
		if !c.epochVoters(i).Add(voterID) {
			// Already voted for the epoch index, so no need to continue further
			break
		}
	}

	c.Events.VotersUpdated.Trigger(&VoterUpdatedEvent{
		Voter:                voter,
		NewLatestEpochIndex:  epochIndex,
		PrevLatestEpochIndex: previousHighestIndex,
	})
}

func (c *CommitmentTracker) Voters(epochIndex epoch.Index) (voters *validator.Set) {
	voters = validator.NewSet()
	epochVoters := c.epochVoters(epochIndex)
	if epochVoters.IsEmpty() {
		return
	}

	epochVoters.ForEach(func(identityID identity.ID) error {
		voter, validatorExists := c.validatorSet.Get(identityID)
		if validatorExists {
			voters.Add(voter)
		}
		return nil
	})

	return
}

func (c *CommitmentTracker) EvictEpoch(indexToEvict epoch.Index) {
	identities, exists := c.votersPerEpoch.Get(indexToEvict)
	if !exists {
		return
	}

	var identitiesToPrune []identity.ID
	identities.ForEach(func(identity identity.ID) error {
		votesForIdentity, has := c.votesPerIdentity.Get(identity)
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
		c.votesPerIdentity.Delete(identity)
	}

	c.votersPerEpoch.Delete(indexToEvict)
}

// region CommitmentVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type CommitmentVotePower struct {
	Index epoch.Index
}

func (p CommitmentVotePower) Compare(other CommitmentVotePower) int {
	if p.Index-other.Index < 0 {
		return -1
	} else if p.Index-other.Index > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
