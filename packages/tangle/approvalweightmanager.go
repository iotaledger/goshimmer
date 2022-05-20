package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

const (
	minVoterWeight float64 = 0.000000000000001
)

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

// ApprovalWeightManager is a Tangle component to keep track of relative weights of branches and markers so that
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
	a.tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *MessageBookedEvent) {
		a.processBookedMessage(event.MessageID)
	}))
	a.tangle.Booker.Events.MessageBranchUpdated.Hook(event.NewClosure(func(event *MessageBranchUpdatedEvent) {
		a.processForkedMessage(event.MessageID, event.BranchID)
	}))
	a.tangle.Booker.Events.MarkerBranchAdded.Hook(event.NewClosure(func(event *MarkerBranchAddedEvent) {
		a.processForkedMarker(event.Marker, event.NewBranchID)
	}))
}

// processBookedMessage is the main entry point for the ApprovalWeightManager. It takes the Message's issuer, adds it to the
// voters of the Message's ledger.Branch and approved markers.Marker and eventually triggers events when
// approval weights for branch and markers are reached.
func (a *ApprovalWeightManager) processBookedMessage(messageID MessageID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.updateBranchVoters(message)
		a.updateSequenceVoters(message)

		a.Events.MessageProcessed.Trigger(&MessageProcessedEvent{messageID})
	})
}

// WeightOfBranch returns the weight of the given Branch that was added by Voters of the given epoch.
func (a *ApprovalWeightManager) WeightOfBranch(branchID utxo.TransactionID) (weight float64) {
	a.tangle.Storage.BranchWeight(branchID).Consume(func(branchWeight *BranchWeight) {
		weight = branchWeight.Weight()
	})

	return
}

// WeightOfMarker returns the weight of the given marker based on the anchorTime.
func (a *ApprovalWeightManager) WeightOfMarker(marker *markers.Marker, anchorTime time.Time) (weight float64) {
	activeWeight, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()

	voterWeight := float64(0)
	for voter := range a.markerVotes(marker) {
		voterWeight += activeWeight[voter]
	}

	return voterWeight / totalWeight
}

// Shutdown shuts down the ApprovalWeightManager and persists its state.
func (a *ApprovalWeightManager) Shutdown() {}

func (a *ApprovalWeightManager) isRelevantVoter(message *Message) bool {
	voterWeight, totalWeight := a.tangle.WeightProvider.Weight(message)

	return voterWeight/totalWeight >= minVoterWeight
}

// VotersOfBranch returns the Voters of the given branch ledger.BranchID.
func (a *ApprovalWeightManager) VotersOfBranch(branchID utxo.TransactionID) (voters *Voters) {
	if !a.tangle.Storage.BranchVoters(branchID).Consume(func(branchVoters *BranchVoters) {
		voters = branchVoters.Voters()
	}) {
		voters = NewVoters()
	}
	return
}

// markerVotes returns a map containing Voters associated to their respective SequenceNumbers.
func (a *ApprovalWeightManager) markerVotes(marker *markers.Marker) (markerVotes map[Voter]uint64) {
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

func (a *ApprovalWeightManager) updateBranchVoters(message *Message) {
	branchesOfMessage, err := a.tangle.Booker.MessageBranchIDs(message.ID())
	if err != nil {
		panic(err)
	}

	voter := identity.NewID(message.IssuerPublicKey())
	vote := &BranchVote{
		Voter:     voter,
		VotePower: message.SequenceNumber(),
	}

	addedBranchIDs, revokedBranchIDs, isInvalid := a.determineVotes(branchesOfMessage, vote)
	if isInvalid {
		a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetSubjectivelyInvalid(true)
		})
	}

	if !a.isRelevantVoter(message) {
		return
	}

	addedVote := vote.WithOpinion(Confirmed)
	for it := addedBranchIDs.Iterator(); it.HasNext(); {
		addBranchID := it.Next()
		a.addVoterToBranch(addBranchID, addedVote.WithBranchID(addBranchID))
	}

	revokedVote := vote.WithOpinion(Rejected)
	for it := revokedBranchIDs.Iterator(); it.HasNext(); {
		revokedBranchID := it.Next()
		a.revokeVoterFromBranch(revokedBranchID, revokedVote.WithBranchID(revokedBranchID))
	}
}

// determineVotes iterates over a set of branches and, taking into account the opinion a Voter expressed previously,
// computes the branches that will receive additional weight, the ones that will see their weight revoked, and if the
// result constitutes an overrall valid state transition.
func (a *ApprovalWeightManager) determineVotes(votedBranchIDs *set.AdvancedSet[utxo.TransactionID], vote *BranchVote) (addedBranches, revokedBranches *set.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	addedBranches = set.NewAdvancedSet[utxo.TransactionID]()
	for it := votedBranchIDs.Iterator(); it.HasNext(); {
		votedBranchID := it.Next()
		conflictingBranchWithHigherVoteExists := false
		a.tangle.Ledger.ConflictDAG.Utils.ForEachConflictingBranchID(votedBranchID, func(conflictingBranchID utxo.TransactionID) bool {
			conflictingBranchWithHigherVoteExists = a.identicalVoteWithHigherPowerExists(vote.WithBranchID(conflictingBranchID).WithOpinion(Confirmed))

			return !conflictingBranchWithHigherVoteExists
		})

		if conflictingBranchWithHigherVoteExists {
			continue
		}

		// The starting branches should not be considered as having common Parents, hence we treat them separately.
		conflictAddedBranches, _ := a.determineBranchesToAdd(set.NewAdvancedSet(votedBranchID), vote.WithOpinion(Confirmed))
		addedBranches.AddAll(conflictAddedBranches)
	}
	revokedBranches, isInvalid = a.determineBranchesToRevoke(addedBranches, votedBranchIDs, vote.WithOpinion(Rejected))

	return
}

// determineBranchesToAdd iterates through the past cone of the given Branches and determines the BranchIDs that
// are affected by the Vote.
func (a *ApprovalWeightManager) determineBranchesToAdd(branchIDs *set.AdvancedSet[utxo.TransactionID], branchVote *BranchVote) (addedBranches *set.AdvancedSet[utxo.TransactionID], allParentsAdded bool) {
	addedBranches = set.NewAdvancedSet[utxo.TransactionID]()

	for it := branchIDs.Iterator(); it.HasNext(); {
		currentBranchID := it.Next()
		currentVote := branchVote.WithBranchID(currentBranchID)

		// Do not queue parents if a newer vote exists for this branch for this voter.
		if a.identicalVoteWithHigherPowerExists(currentVote) {
			continue
		}

		a.tangle.Ledger.ConflictDAG.Storage.CachedBranch(currentBranchID).Consume(func(branch *branchdag.Branch[utxo.TransactionID, utxo.OutputID]) {
			addedBranchesOfCurrentBranch, allParentsOfCurrentBranchAdded := a.determineBranchesToAdd(branch.Parents(), branchVote)
			allParentsAdded = allParentsAdded && allParentsOfCurrentBranchAdded

			addedBranches.AddAll(addedBranchesOfCurrentBranch)
		})

		addedBranches.Add(currentBranchID)
	}

	return
}

// determineBranchesToRevoke determines which Branches of the conflicting future cone of the added Branches are affected
// by the vote and if the vote is valid (not voting for conflicting Branches).
func (a *ApprovalWeightManager) determineBranchesToRevoke(addedBranches, votedBranches *set.AdvancedSet[utxo.TransactionID], vote *BranchVote) (revokedBranches *set.AdvancedSet[utxo.TransactionID], isInvalid bool) {
	revokedBranches = set.NewAdvancedSet[utxo.TransactionID]()
	subTractionWalker := walker.New[utxo.TransactionID]()
	for it := addedBranches.Iterator(); it.HasNext(); {
		a.tangle.Ledger.ConflictDAG.Utils.ForEachConflictingBranchID(it.Next(), func(conflictingBranchID utxo.TransactionID) bool {
			subTractionWalker.Push(conflictingBranchID)

			return true
		})
	}

	for subTractionWalker.HasNext() {
		currentVote := vote.WithBranchID(subTractionWalker.Next())

		if isInvalid = addedBranches.Has(currentVote.BranchID) || votedBranches.Has(currentVote.BranchID); isInvalid {
			return
		}

		revokedBranches.Add(currentVote.BranchID)

		a.tangle.Ledger.ConflictDAG.Storage.CachedChildBranches(currentVote.BranchID).Consume(func(childBranch *branchdag.ChildBranch[utxo.TransactionID]) {
			subTractionWalker.Push(childBranch.ChildBranchID())
		})
	}

	return
}

func (a *ApprovalWeightManager) identicalVoteWithHigherPowerExists(vote *BranchVote) (exists bool) {
	existingVote, exists := a.voteWithHigherPower(vote)

	return exists && vote.Opinion == existingVote.Opinion
}

func (a *ApprovalWeightManager) voteWithHigherPower(vote *BranchVote) (existingVote *BranchVote, exists bool) {
	a.tangle.Storage.LatestBranchVotes(vote.Voter).Consume(func(latestBranchVotes *LatestBranchVotes) {
		existingVote, exists = latestBranchVotes.Vote(vote.BranchID)
	})

	return existingVote, exists && existingVote.VotePower > vote.VotePower
}

func (a *ApprovalWeightManager) addVoterToBranch(branchID utxo.TransactionID, branchVote *BranchVote) {
	a.tangle.Storage.LatestBranchVotes(branchVote.Voter, NewLatestBranchVotes).Consume(func(latestBranchVotes *LatestBranchVotes) {
		latestBranchVotes.Store(branchVote)
	})

	a.tangle.Storage.BranchVoters(branchID, NewBranchVoters).Consume(func(branchVoters *BranchVoters) {
		branchVoters.AddVoter(branchVote.Voter)
	})

	a.updateBranchWeight(branchID)
}

func (a *ApprovalWeightManager) revokeVoterFromBranch(branchID utxo.TransactionID, branchVote *BranchVote) {
	a.tangle.Storage.LatestBranchVotes(branchVote.Voter, NewLatestBranchVotes).Consume(func(latestBranchVotes *LatestBranchVotes) {
		latestBranchVotes.Store(branchVote)
	})

	a.tangle.Storage.BranchVoters(branchID, NewBranchVoters).Consume(func(branchVoters *BranchVoters) {
		branchVoters.DeleteVoter(branchVote.Voter)
	})

	a.updateBranchWeight(branchID)
}

func (a *ApprovalWeightManager) updateSequenceVoters(message *Message) {
	if !a.isRelevantVoter(message) {
		return
	}

	a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// Do not revisit markers that have already been visited. With the like switch there can be cycles in the sequence DAG
		// which results in endless walks.
		supportWalker := walker.New[markers.Marker](false)

		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			supportWalker.Push(*markers.NewMarker(sequenceID, index))

			return true
		})

		for supportWalker.HasNext() {
			a.addVoteToMarker(supportWalker.Next(), message, supportWalker)
		}
	})
}

func (a *ApprovalWeightManager) addVoteToMarker(marker markers.Marker, message *Message, walk *walker.Walker[markers.Marker]) {
	// We don't add the voter and abort if the marker is already confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new branch once a conflict arrives and we copy over the
	// voters of the marker to the branch. Since the marker is already seen as confirmed it should not matter too much though.
	if a.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(marker.SequenceID()) >= marker.Index() {
		return
	}

	a.tangle.Storage.LatestMarkerVotes(marker.SequenceID(), identity.NewID(message.IssuerPublicKey()), NewLatestMarkerVotes).Consume(func(latestMarkerVotes *LatestMarkerVotes) {
		stored, previousHighestIndex := latestMarkerVotes.Store(marker.Index(), message.SequenceNumber())
		if !stored {
			return
		}

		if marker.Index() > previousHighestIndex {
			a.updateMarkerWeights(marker.SequenceID(), previousHighestIndex+1, marker.Index())
		}

		a.tangle.Booker.MarkersManager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
			sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
				walk.Push(*markers.NewMarker(sequenceID, index))

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
		if a.tangle.Booker.MarkersManager.MessageID(currentMarker) == EmptyMessageID {
			continue
		}

		voterWeight := float64(0)
		for voter := range a.markerVotes(currentMarker) {
			voterWeight += activeWeights[voter]
		}

		a.Events.MarkerWeightChanged.Trigger(&MarkerWeightChangedEvent{currentMarker, voterWeight / totalWeight})
	}
}

func (a *ApprovalWeightManager) updateBranchWeight(branchID utxo.TransactionID) {
	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()

	var voterWeight float64
	a.VotersOfBranch(branchID).ForEach(func(voter Voter) {
		voterWeight += activeWeights[voter]
	})

	newBranchWeight := voterWeight / totalWeight

	a.tangle.Storage.BranchWeight(branchID, NewBranchWeight).Consume(func(branchWeight *BranchWeight) {
		if !branchWeight.SetWeight(newBranchWeight) {
			return
		}

		a.Events.BranchWeightChanged.Trigger(&BranchWeightChangedEvent{branchID, newBranchWeight})
	})
}

// processForkedMessage updates the Branch weight after an individually mapped Message was forked into a new Branch.
func (a *ApprovalWeightManager) processForkedMessage(messageID MessageID, forkedBranchID utxo.TransactionID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.tangle.Storage.BranchVoters(forkedBranchID, NewBranchVoters).Consume(func(forkedBranchVoters *BranchVoters) {
			a.tangle.Ledger.ConflictDAG.Storage.CachedBranch(forkedBranchID).Consume(func(forkedBranch *branchdag.Branch[utxo.TransactionID, utxo.OutputID]) {
				if !a.addSupportToForkedBranchVoters(identity.NewID(message.IssuerPublicKey()), forkedBranchVoters, forkedBranch.Parents(), message.SequenceNumber()) {
					return
				}

				a.updateBranchWeight(forkedBranchID)
			})
		})
	})
}

// take everything in future cone because it was not conflicting before and move to new branch.
func (a *ApprovalWeightManager) processForkedMarker(marker *markers.Marker, forkedBranchID utxo.TransactionID) {
	branchVotesUpdated := false
	a.tangle.Storage.BranchVoters(forkedBranchID, NewBranchVoters).Consume(func(branchVoters *BranchVoters) {
		a.tangle.Ledger.ConflictDAG.Storage.CachedBranch(forkedBranchID).Consume(func(forkedBranch *branchdag.Branch[utxo.TransactionID, utxo.OutputID]) {
			// If we want to add the branchVoters to the newly-forker branch, we have to make sure the
			// voters of the marker we are forking also voted for all parents of the branch the marker is
			// being forked into.
			parentBranchIDs := forkedBranch.Parents()

			for voter, sequenceNumber := range a.markerVotes(marker) {
				if !a.addSupportToForkedBranchVoters(voter, branchVoters, parentBranchIDs, sequenceNumber) {
					continue
				}

				branchVotesUpdated = true
			}
		})
	})

	if !branchVotesUpdated {
		return
	}

	a.updateBranchWeight(forkedBranchID)
}

func (a *ApprovalWeightManager) addSupportToForkedBranchVoters(voter Voter, forkedBranchVoters *BranchVoters, parentBranchIDs *set.AdvancedSet[utxo.TransactionID], sequenceNumber uint64) (supportAdded bool) {
	if !a.voterSupportsAllBranches(voter, parentBranchIDs) {
		return false
	}

	a.tangle.Storage.LatestBranchVotes(voter, NewLatestBranchVotes).Consume(func(latestBranchVotes *LatestBranchVotes) {
		supportAdded = latestBranchVotes.Store(&BranchVote{
			Voter:     voter,
			BranchID:  forkedBranchVoters.BranchID(),
			Opinion:   Confirmed,
			VotePower: sequenceNumber,
		})
	})

	return supportAdded && forkedBranchVoters.AddVoter(voter)
}

func (a *ApprovalWeightManager) voterSupportsAllBranches(voter Voter, branchIDs *set.AdvancedSet[utxo.TransactionID]) (allBranchesSupported bool) {
	allBranchesSupported = true
	for it := branchIDs.Iterator(); it.HasNext(); {
		a.tangle.Storage.BranchVoters(it.Next()).Consume(func(branchVoters *BranchVoters) {
			allBranchesSupported = branchVoters.Has(voter)
		})

		if !allBranchesSupported {
			return false
		}
	}

	return allBranchesSupported
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
