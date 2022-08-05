package tangleold

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

const (
	minVoterWeight float64 = 0.000000000000001
)

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

// ApprovalWeightManager is a Tangle component to keep track of relative weights of conflicts and markers so that
// consensus can be based on the heaviest perception on the tangle as a data structure.
type ApprovalWeightManager struct {
	Events *ApprovalWeightManagerEvents
	tangle *Tangle
}

// NewApprovalWeightManager is the constructor for ApprovalWeightManager.
func NewApprovalWeightManager(tangle *Tangle) (approvalWeightManager *ApprovalWeightManager) {
	approvalWeightManager = &ApprovalWeightManager{
		Events: newApprovalWeightManagerEvents(),
		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (a *ApprovalWeightManager) Setup() {
	a.tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *BlockBookedEvent) {
		a.processBookedBlock(event.BlockID)
	}))
	a.tangle.Booker.Events.BlockConflictUpdated.Hook(event.NewClosure(func(event *BlockConflictUpdatedEvent) {
		a.processForkedBlock(event.BlockID, event.ConflictID)
	}))
	a.tangle.Booker.Events.MarkerConflictAdded.Hook(event.NewClosure(func(event *MarkerConflictAddedEvent) {
		a.processForkedMarker(event.Marker, event.NewConflictID)
	}))
}

// processBookedBlock is the main entry point for the ApprovalWeightManager. It takes the Block's issuer, adds it to the
// voters of the Block's ledger.Conflict and approved markers.Marker and eventually triggers events when
// approval weights for conflict and markers are reached.
func (a *ApprovalWeightManager) processBookedBlock(blockID BlockID) {
	a.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		a.updateConflictVoters(block)
		a.updateSequenceVoters(block)

		a.Events.BlockProcessed.Trigger(&BlockProcessedEvent{blockID})
	})
}

// WeightOfConflict returns the weight of the given Conflict that was added by Voters of the given epoch.
func (a *ApprovalWeightManager) WeightOfConflict(conflictID utxo.TransactionID) (weight float64) {
	a.tangle.Storage.ConflictWeight(conflictID).Consume(func(conflictWeight *ConflictWeight) {
		weight = conflictWeight.Weight()
	})

	return
}

// WeightOfMarker returns the weight of the given marker based on the anchorTime.
func (a *ApprovalWeightManager) WeightOfMarker(marker markers.Marker, anchorTime time.Time) (weight float64) {
	activeWeight, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()

	voterWeight := float64(0)
	for voter := range a.markerVotes(marker) {
		voterWeight += activeWeight[voter]
	}

	return voterWeight / totalWeight
}

// Shutdown shuts down the ApprovalWeightManager and persists its state.
func (a *ApprovalWeightManager) Shutdown() {}

func (a *ApprovalWeightManager) isRelevantVoter(block *Block) bool {
	voterWeight, totalWeight := a.tangle.WeightProvider.Weight(block)

	return voterWeight/totalWeight >= minVoterWeight
}

// VotersOfConflict returns the Voters of the given conflict ledger.ConflictID.
func (a *ApprovalWeightManager) VotersOfConflict(conflictID utxo.TransactionID) (voters *Voters) {
	if !a.tangle.Storage.ConflictVoters(conflictID).Consume(func(conflictVoters *ConflictVoters) {
		voters = conflictVoters.Voters()
	}) {
		voters = NewVoters()
	}
	return
}

// markerVotes returns a map containing Voters associated to their respective SequenceNumbers.
func (a *ApprovalWeightManager) markerVotes(marker markers.Marker) (markerVotes map[Voter]uint64) {
	markerVotes = make(map[Voter]uint64)
	a.tangle.Storage.AllLatestMarkerVotes(marker.SequenceID()).Consume(func(latestMarkerVotes *LatestMarkerVotes) {
		lastPower, exists := latestMarkerVotes.Power(marker.Index())
		if !exists {
			return
		}

		markerVotes[latestMarkerVotes.Voter()] = lastPower
	})

	return markerVotes
}

func (a *ApprovalWeightManager) updateConflictVoters(block *Block) {
	conflictsOfBlock, err := a.tangle.Booker.BlockConflictIDs(block.ID())
	if err != nil {
		panic(err)
	}

	voter := identity.NewID(block.IssuerPublicKey())

	// create vote with default ConflictID and Opinion values that will be filled later
	vote := NewConflictVote(voter, block.SequenceNumber(), utxo.TransactionID{}, UndefinedOpinion)

	addedConflictIDs, revokedConflictIDs, isInvalid := a.determineVotes(conflictsOfBlock, vote)
	if isInvalid {
		a.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
			blockMetadata.SetSubjectivelyInvalid(true)
		})
		return
	}

	if !a.isRelevantVoter(block) {
		return
	}

	addedVote := vote.WithOpinion(Confirmed)
	for it := addedConflictIDs.Iterator(); it.HasNext(); {
		addConflictID := it.Next()
		a.addVoterToConflict(addConflictID, addedVote.WithConflictID(addConflictID))
	}

	revokedVote := vote.WithOpinion(Rejected)
	for it := revokedConflictIDs.Iterator(); it.HasNext(); {
		revokedConflictID := it.Next()
		a.revokeVoterFromConflict(revokedConflictID, revokedVote.WithConflictID(revokedConflictID))
	}
}

// determineVotes iterates over a set of conflicts and, taking into account the opinion a Voter expressed previously,
// computes the conflicts that will receive additional weight, the ones that will see their weight revoked, and if the
// result constitutes an overrall valid state transition.
func (a *ApprovalWeightManager) determineVotes(votedConflictIDs *set.AdvancedSet[utxo.TransactionID], vote *ConflictVote) (addedConflicts, revokedConflicts *set.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	addedConflicts = utxo.NewTransactionIDs()
	for it := votedConflictIDs.Iterator(); it.HasNext(); {
		votedConflictID := it.Next()

		conflictingConflictWithHigherVoteExists := false
		a.tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(votedConflictID, func(conflictingConflictID utxo.TransactionID) bool {
			conflictingConflictWithHigherVoteExists = a.identicalVoteWithHigherPowerExists(vote.WithConflictID(conflictingConflictID).WithOpinion(Confirmed))

			return !conflictingConflictWithHigherVoteExists
		})

		if conflictingConflictWithHigherVoteExists {
			continue
		}

		// The starting conflicts should not be considered as having common Parents, hence we treat them separately.
		conflictAddedConflicts, _ := a.determineConflictsToAdd(set.NewAdvancedSet(votedConflictID), vote.WithOpinion(Confirmed))
		addedConflicts.AddAll(conflictAddedConflicts)
	}
	revokedConflicts, isInvalid = a.determineConflictsToRevoke(addedConflicts, votedConflictIDs, vote.WithOpinion(Rejected))

	return
}

// determineConflictsToAdd iterates through the past cone of the given Conflicts and determines the ConflictIDs that
// are affected by the Vote.
func (a *ApprovalWeightManager) determineConflictsToAdd(conflictIDs *set.AdvancedSet[utxo.TransactionID], conflictVote *ConflictVote) (addedConflicts *set.AdvancedSet[utxo.TransactionID], allParentsAdded bool) {
	addedConflicts = set.NewAdvancedSet[utxo.TransactionID]()

	for it := conflictIDs.Iterator(); it.HasNext(); {
		currentConflictID := it.Next()
		// currentVote := conflictVote.WithConflictID(currentConflictID)

		// Do not queue parents if a newer vote exists for this conflict for this voter.
		// TODO: do not exit earlier to always determine subjectively invalid blocks correctly
		// if a.identicalVoteWithHigherPowerExists(currentVote) {
		// 	continue
		// }

		a.tangle.Ledger.ConflictDAG.Storage.CachedConflict(currentConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			addedConflictsOfCurrentConflict, allParentsOfCurrentConflictAdded := a.determineConflictsToAdd(conflict.Parents(), conflictVote)
			allParentsAdded = allParentsAdded && allParentsOfCurrentConflictAdded

			addedConflicts.AddAll(addedConflictsOfCurrentConflict)
		})

		addedConflicts.Add(currentConflictID)
	}

	return
}

// determineConflictsToRevoke determines which Conflicts of the conflicting future cone of the added Conflicts are affected
// by the vote and if the vote is valid (not voting for conflicting Conflicts).
func (a *ApprovalWeightManager) determineConflictsToRevoke(addedConflicts, votedConflicts *set.AdvancedSet[utxo.TransactionID], vote *ConflictVote) (revokedConflicts *set.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	revokedConflicts = set.NewAdvancedSet[utxo.TransactionID]()
	subTractionWalker := walker.New[utxo.TransactionID]()
	for it := addedConflicts.Iterator(); it.HasNext(); {
		a.tangle.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(it.Next(), func(conflictingConflictID utxo.TransactionID) bool {
			subTractionWalker.Push(conflictingConflictID)

			return true
		})
	}

	for subTractionWalker.HasNext() {
		currentVote := vote.WithConflictID(subTractionWalker.Next())

		if isInvalid = addedConflicts.Has(currentVote.ConflictID()) || votedConflicts.Has(currentVote.ConflictID()); isInvalid {
			return
		}

		revokedConflicts.Add(currentVote.ConflictID())

		a.tangle.Ledger.ConflictDAG.Storage.CachedChildConflicts(currentVote.ConflictID()).Consume(func(childConflict *conflictdag.ChildConflict[utxo.TransactionID]) {
			subTractionWalker.Push(childConflict.ChildConflictID())
		})
	}

	return
}

func (a *ApprovalWeightManager) identicalVoteWithHigherPowerExists(vote *ConflictVote) (exists bool) {
	existingVote, exists := a.voteWithHigherPower(vote)

	return exists && vote.Opinion() == existingVote.Opinion()
}

func (a *ApprovalWeightManager) voteWithHigherPower(vote *ConflictVote) (existingVote *ConflictVote, exists bool) {
	a.tangle.Storage.LatestConflictVotes(vote.Voter()).Consume(func(latestConflictVotes *LatestConflictVotes) {
		existingVote, exists = latestConflictVotes.Vote(vote.ConflictID())
	})

	return existingVote, exists && existingVote.VotePower() > vote.VotePower()
}

func (a *ApprovalWeightManager) addVoterToConflict(conflictID utxo.TransactionID, conflictVote *ConflictVote) {
	a.tangle.Storage.LatestConflictVotes(conflictVote.Voter(), NewLatestConflictVotes).Consume(func(latestConflictVotes *LatestConflictVotes) {
		if existingVote, exists := latestConflictVotes.Vote(conflictID); !exists || existingVote.VotePower() < conflictVote.VotePower() {
			latestConflictVotes.Store(conflictVote)

			a.tangle.Storage.ConflictVoters(conflictID, NewConflictVoters).Consume(func(conflictVoters *ConflictVoters) {
				conflictVoters.AddVoter(conflictVote.Voter())
			})

			a.updateConflictWeight(conflictID)
		}
	})
}

func (a *ApprovalWeightManager) revokeVoterFromConflict(conflictID utxo.TransactionID, conflictVote *ConflictVote) {
	a.tangle.Storage.LatestConflictVotes(conflictVote.Voter(), NewLatestConflictVotes).Consume(func(latestConflictVotes *LatestConflictVotes) {
		if existingVote, exists := latestConflictVotes.Vote(conflictID); !exists || existingVote.VotePower() < conflictVote.VotePower() {
			latestConflictVotes.Store(conflictVote)

			a.tangle.Storage.ConflictVoters(conflictID, NewConflictVoters).Consume(func(conflictVoters *ConflictVoters) {
				conflictVoters.DeleteVoter(conflictVote.Voter())
			})

			a.updateConflictWeight(conflictID)
		}
	})
}

func (a *ApprovalWeightManager) updateSequenceVoters(block *Block) {
	if !a.isRelevantVoter(block) {
		return
	}

	a.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
		// Do not revisit markers that have already been visited. With the like switch there can be cycles in the sequence DAG
		// which results in endless walks.
		supportWalker := walker.New[markers.Marker](false)

		blockMetadata.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			supportWalker.Push(markers.NewMarker(sequenceID, index))

			return true
		})

		for supportWalker.HasNext() {
			a.addVoteToMarker(supportWalker.Next(), block, supportWalker)
		}
	})
}

func (a *ApprovalWeightManager) addVoteToMarker(marker markers.Marker, block *Block, walk *walker.Walker[markers.Marker]) {
	// We don't add the voter and abort if the marker is already confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new conflict once a conflict arrives and we copy over the
	// voters of the marker to the conflict. Since the marker is already seen as confirmed it should not matter too much though.
	if a.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(marker.SequenceID()) >= marker.Index() {
		return
	}

	a.tangle.Storage.LatestMarkerVotes(marker.SequenceID(), identity.NewID(block.IssuerPublicKey()), NewLatestMarkerVotes).Consume(func(latestMarkerVotes *LatestMarkerVotes) {
		stored, previousHighestIndex := latestMarkerVotes.Store(marker.Index(), block.SequenceNumber())
		if !stored {
			return
		}

		if marker.Index() > previousHighestIndex {
			a.updateMarkerWeights(marker.SequenceID(), previousHighestIndex+1, marker.Index())
		}

		a.tangle.Booker.MarkersManager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
			sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
				walk.Push(markers.NewMarker(sequenceID, index))

				return true
			})
		})
	})
}

// updateMarkerWeights updates the marker weights in the given range and triggers the MarkerWeightChanged event.
func (a *ApprovalWeightManager) updateMarkerWeights(sequenceID markers.SequenceID, rangeStartIndex, rangeEndIndex markers.Index) {
	if rangeStartIndex <= 1 {
		rangeStartIndex = a.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(sequenceID)
	}

	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()
	for i := rangeStartIndex; i <= rangeEndIndex; i++ {
		currentMarker := markers.NewMarker(sequenceID, i)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if a.tangle.Booker.MarkersManager.BlockID(currentMarker) == EmptyBlockID {
			continue
		}

		voterWeight := float64(0)
		for voter := range a.markerVotes(currentMarker) {
			voterWeight += activeWeights[voter]
		}

		a.Events.MarkerWeightChanged.Trigger(&MarkerWeightChangedEvent{currentMarker, voterWeight / totalWeight})
	}
}

func (a *ApprovalWeightManager) updateConflictWeight(conflictID utxo.TransactionID) {
	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()

	var voterWeight float64
	a.VotersOfConflict(conflictID).Set.ForEach(func(voter Voter) {
		voterWeight += activeWeights[voter]
	})

	newConflictWeight := voterWeight / totalWeight

	a.tangle.Storage.ConflictWeight(conflictID, NewConflictWeight).Consume(func(conflictWeight *ConflictWeight) {
		if !conflictWeight.SetWeight(newConflictWeight) {
			return
		}

		a.Events.ConflictWeightChanged.Trigger(&ConflictWeightChangedEvent{conflictID, newConflictWeight})
	})
}

// processForkedBlock updates the Conflict weight after an individually mapped Block was forked into a new Conflict.
func (a *ApprovalWeightManager) processForkedBlock(blockID BlockID, forkedConflictID utxo.TransactionID) {
	a.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		a.tangle.Storage.ConflictVoters(forkedConflictID, NewConflictVoters).Consume(func(forkedConflictVoters *ConflictVoters) {
			a.tangle.Ledger.ConflictDAG.Storage.CachedConflict(forkedConflictID).Consume(func(forkedConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				if !a.addSupportToForkedConflictVoters(identity.NewID(block.IssuerPublicKey()), forkedConflictVoters, forkedConflict.Parents(), block.SequenceNumber()) {
					return
				}

				a.updateConflictWeight(forkedConflictID)
			})
		})
	})
}

// take everything in future cone because it was not conflicting before and move to new conflict.
func (a *ApprovalWeightManager) processForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID) {
	conflictVotesUpdated := false
	a.tangle.Storage.ConflictVoters(forkedConflictID, NewConflictVoters).Consume(func(conflictVoters *ConflictVoters) {
		a.tangle.Ledger.ConflictDAG.Storage.CachedConflict(forkedConflictID).Consume(func(forkedConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			// If we want to add the conflictVoters to the newly-forker conflict, we have to make sure the
			// voters of the marker we are forking also voted for all parents of the conflict the marker is
			// being forked into.
			parentConflictIDs := forkedConflict.Parents()

			for voter, sequenceNumber := range a.markerVotes(marker) {
				if !a.addSupportToForkedConflictVoters(voter, conflictVoters, parentConflictIDs, sequenceNumber) {
					continue
				}

				conflictVotesUpdated = true
			}
		})
	})

	if !conflictVotesUpdated {
		return
	}

	a.updateConflictWeight(forkedConflictID)
}

func (a *ApprovalWeightManager) addSupportToForkedConflictVoters(voter Voter, forkedConflictVoters *ConflictVoters, parentConflictIDs *set.AdvancedSet[utxo.TransactionID], sequenceNumber uint64) (supportAdded bool) {
	if !a.voterSupportsAllConflicts(voter, parentConflictIDs) {
		return false
	}

	a.tangle.Storage.LatestConflictVotes(voter, NewLatestConflictVotes).Consume(func(latestConflictVotes *LatestConflictVotes) {
		supportAdded = latestConflictVotes.Store(NewConflictVote(voter, sequenceNumber, forkedConflictVoters.ConflictID(), Confirmed))
	})

	return supportAdded && forkedConflictVoters.AddVoter(voter)
}

func (a *ApprovalWeightManager) voterSupportsAllConflicts(voter Voter, conflictIDs *set.AdvancedSet[utxo.TransactionID]) (allConflictsSupported bool) {
	allConflictsSupported = true
	for it := conflictIDs.Iterator(); it.HasNext(); {
		a.tangle.Storage.ConflictVoters(it.Next()).Consume(func(conflictVoters *ConflictVoters) {
			allConflictsSupported = conflictVoters.Has(voter)
		})

		if !allConflictsSupported {
			return false
		}
	}

	return allConflictsSupported
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
