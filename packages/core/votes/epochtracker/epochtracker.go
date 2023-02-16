package epochtracker

import (
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/votes/latestvotes"
)

type EpochTracker struct {
	Events *Events

	votesPerIdentity *memstorage.Storage[identity.ID, *latestvotes.LatestVotes[epoch.Index, EpochVotePower]]
	votersPerEpoch   *memstorage.Storage[epoch.Index, *advancedset.AdvancedSet[identity.ID]]

	cutoffIndexCallback func() epoch.Index
}

func NewEpochTracker(cutoffIndexCallback func() epoch.Index) *EpochTracker {
	return &EpochTracker{
		votesPerIdentity: memstorage.New[identity.ID, *latestvotes.LatestVotes[epoch.Index, EpochVotePower]](),
		votersPerEpoch:   memstorage.New[epoch.Index, *advancedset.AdvancedSet[identity.ID]](),

		cutoffIndexCallback: cutoffIndexCallback,
		Events:              NewEvents(),
	}
}

func (c *EpochTracker) epochVoters(epochIndex epoch.Index) *advancedset.AdvancedSet[identity.ID] {
	epochVoters, _ := c.votersPerEpoch.RetrieveOrCreate(epochIndex, func() *advancedset.AdvancedSet[identity.ID] {
		return advancedset.NewAdvancedSet[identity.ID]()
	})
	return epochVoters
}

func (c *EpochTracker) TrackVotes(epochIndex epoch.Index, voterID identity.ID, power EpochVotePower) {
	epochVoters := c.epochVoters(epochIndex)
	if epochVoters.Has(voterID) {
		// We already tracked the voter for this epoch, so no need to update anything
		return
	}

	votersVotes, _ := c.votesPerIdentity.RetrieveOrCreate(voterID, func() *latestvotes.LatestVotes[epoch.Index, EpochVotePower] {
		return latestvotes.NewLatestVotes[epoch.Index, EpochVotePower](voterID)
	})

	updated, previousHighestIndex := votersVotes.Store(epochIndex, power)
	if !updated || previousHighestIndex >= epochIndex {
		return
	}

	for i := lo.Max(c.cutoffIndexCallback(), previousHighestIndex) + 1; i <= epochIndex; i++ {
		c.epochVoters(i).Add(voterID)
	}

	c.Events.VotersUpdated.Trigger(&VoterUpdatedEvent{
		Voter:                voterID,
		NewLatestEpochIndex:  epochIndex,
		PrevLatestEpochIndex: previousHighestIndex,
	})
}

func (c *EpochTracker) Voters(epochIndex epoch.Index) (voters *advancedset.AdvancedSet[identity.ID]) {
	voters = advancedset.NewAdvancedSet[identity.ID]()

	epochVoters, exists := c.votersPerEpoch.Get(epochIndex)
	if !exists {
		return voters
	}

	_ = epochVoters.ForEach(func(identityID identity.ID) error {
		voters.Add(identityID)
		return nil
	})

	return voters
}

func (c *EpochTracker) EvictEpoch(indexToEvict epoch.Index) {
	identities, exists := c.votersPerEpoch.Get(indexToEvict)
	if !exists {
		return
	}

	var identitiesToPrune []identity.ID
	_ = identities.ForEach(func(identity identity.ID) error {
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

// region EpochVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type EpochVotePower struct {
	Index epoch.Index
}

func (p EpochVotePower) Compare(other EpochVotePower) int {
	if p.Index-other.Index < 0 {
		return -1
	} else if p.Index-other.Index > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
