package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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
		Events: &ApprovalWeightManagerEvents{
			MessageProcessed:    events.NewEvent(MessageIDCaller),
			MarkerWeightChanged: events.NewEvent(markerWeightChangedEventHandler),
			BranchWeightChanged: events.NewEvent(branchWeightChangedEventHandler),
		},
		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (a *ApprovalWeightManager) Setup() {
	a.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(a.processBookedMessage))
	a.tangle.Booker.Events.MessageBranchUpdated.Attach(events.NewClosure(a.processForkedMessage))
	a.tangle.Booker.Events.MarkerBranchAdded.Attach(events.NewClosure(a.processForkedMarker))
}

// processBookedMessage is the main entry point for the ApprovalWeightManager. It takes the Message's issuer, adds it to the
// voters of the Message's ledgerstate.Branch and approved markers.Marker and eventually triggers events when
// approval weights for branch and markers are reached.
func (a *ApprovalWeightManager) processBookedMessage(messageID MessageID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.updateBranchVoters(message)
		a.updateSequenceVoters(message)

		a.Events.MessageProcessed.Trigger(messageID)
	})
}

// WeightOfBranch returns the weight of the given Branch that was added by Voters of the given epoch.
func (a *ApprovalWeightManager) WeightOfBranch(branchID ledgerstate.BranchID) (weight float64) {
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

// VotersOfBranch returns the Voters of the given branch ledgerstate.BranchID.
func (a *ApprovalWeightManager) VotersOfBranch(branchID ledgerstate.BranchID) (voters *Voters) {
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
	for addBranchID := range addedBranchIDs {
		a.addVoterToBranch(addBranchID, addedVote.WithBranchID(addBranchID))
	}

	revokedVote := vote.WithOpinion(Rejected)
	for revokedBranchID := range revokedBranchIDs {
		a.revokeVoterFromBranch(revokedBranchID, revokedVote.WithBranchID(revokedBranchID))
	}
}

// determineVotes iterates over a set of branches and, taking into account the opinion a Voter expressed previously,
// computes the branches that will receive additional weight, the ones that will see their weight revoked, and if the
// result constitutes an overrall valid state transition.
func (a *ApprovalWeightManager) determineVotes(votedBranchIDs ledgerstate.BranchIDs, vote *BranchVote) (addedBranches, revokedBranches ledgerstate.BranchIDs, isInvalid bool) {
	addedBranches = ledgerstate.NewBranchIDs()
	for votedBranchID := range votedBranchIDs {
		conflictingBranchWithHigherVoteExists := false
		a.tangle.LedgerState.ForEachConflictingBranchID(votedBranchID, func(conflictingBranchID ledgerstate.BranchID) bool {
			conflictingBranchWithHigherVoteExists = a.identicalVoteWithHigherPowerExists(vote.WithBranchID(conflictingBranchID).WithOpinion(Confirmed))

			return !conflictingBranchWithHigherVoteExists
		})

		if conflictingBranchWithHigherVoteExists {
			continue
		}

		// The starting branches should not be considered as having common Parents, hence we treat them separately.
		conflictAddedBranches, _ := a.determineBranchesToAdd(ledgerstate.NewBranchIDs(votedBranchID), vote.WithOpinion(Confirmed))
		addedBranches.AddAll(conflictAddedBranches)
	}
	revokedBranches, isInvalid = a.determineBranchesToRevoke(addedBranches, votedBranchIDs, vote.WithOpinion(Rejected))

	return
}

// determineBranchesToAdd iterates through the past cone of the given Branches and determines the BranchIDs that
// are affected by the Vote.
func (a *ApprovalWeightManager) determineBranchesToAdd(branchIDs ledgerstate.BranchIDs, branchVote *BranchVote) (addedBranches ledgerstate.BranchIDs, allParentsAdded bool) {
	addedBranches = ledgerstate.NewBranchIDs()

	for currentBranchID := range branchIDs {
		currentVote := branchVote.WithBranchID(currentBranchID)

		// Do not queue parents if a newer vote exists for this branch for this voter.
		if a.identicalVoteWithHigherPowerExists(currentVote) {
			continue
		}

		a.tangle.LedgerState.Branch(currentBranchID).Consume(func(branch *ledgerstate.Branch) {
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
func (a *ApprovalWeightManager) determineBranchesToRevoke(addedBranches, votedBranches ledgerstate.BranchIDs, vote *BranchVote) (revokedBranches ledgerstate.BranchIDs, isInvalid bool) {
	revokedBranches = ledgerstate.NewBranchIDs()
	subTractionWalker := walker.New[ledgerstate.BranchID]()
	for addedBranch := range addedBranches {
		a.tangle.LedgerState.ForEachConflictingBranchID(addedBranch, func(conflictingBranchID ledgerstate.BranchID) bool {
			subTractionWalker.Push(conflictingBranchID)

			return true
		})
	}

	for subTractionWalker.HasNext() {
		currentVote := vote.WithBranchID(subTractionWalker.Next())

		if isInvalid = addedBranches.Contains(currentVote.BranchID) || votedBranches.Contains(currentVote.BranchID); isInvalid {
			return
		}

		revokedBranches.Add(currentVote.BranchID)

		a.tangle.LedgerState.ChildBranches(currentVote.BranchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
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

func (a *ApprovalWeightManager) addVoterToBranch(branchID ledgerstate.BranchID, branchVote *BranchVote) {
	if branchID == ledgerstate.MasterBranchID {
		return
	}

	a.tangle.Storage.LatestBranchVotes(branchVote.Voter, NewLatestBranchVotes).Consume(func(latestBranchVotes *LatestBranchVotes) {
		latestBranchVotes.Store(branchVote)
	})

	a.tangle.Storage.BranchVoters(branchID, NewBranchVoters).Consume(func(branchVoters *BranchVoters) {
		branchVoters.AddVoter(branchVote.Voter)
	})

	a.updateBranchWeight(branchID)
}

func (a *ApprovalWeightManager) revokeVoterFromBranch(branchID ledgerstate.BranchID, branchVote *BranchVote) {
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

func (a *ApprovalWeightManager) updateBranchWeight(branchID ledgerstate.BranchID) {
	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantVoters()

	var voterWeight float64
	a.VotersOfBranch(branchID).Set.ForEach(func(voter Voter) {
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
func (a *ApprovalWeightManager) processForkedMessage(messageID MessageID, forkedBranchID ledgerstate.BranchID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.tangle.Storage.BranchVoters(forkedBranchID, NewBranchVoters).Consume(func(forkedBranchVoters *BranchVoters) {
			a.tangle.LedgerState.Branch(forkedBranchID).Consume(func(forkedBranch *ledgerstate.Branch) {
				if !a.addSupportToForkedBranchVoters(identity.NewID(message.IssuerPublicKey()), forkedBranchVoters, forkedBranch.Parents(), message.SequenceNumber()) {
					return
				}

				a.updateBranchWeight(forkedBranchID)
			})
		})
	})
}

// take everything in future cone because it was not conflicting before and move to new branch.
func (a *ApprovalWeightManager) processForkedMarker(marker *markers.Marker, oldBranchIDs ledgerstate.BranchIDs, forkedBranchID ledgerstate.BranchID) {
	branchVotesUpdated := false
	a.tangle.Storage.BranchVoters(forkedBranchID, NewBranchVoters).Consume(func(branchVoters *BranchVoters) {
		a.tangle.LedgerState.Branch(forkedBranchID).Consume(func(forkedBranch *ledgerstate.Branch) {
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

	a.tangle.Storage.Message(a.tangle.Booker.MarkersManager.MessageID(marker)).Consume(func(message *Message) {
		a.updateBranchWeight(forkedBranchID)
	})
}

func (a *ApprovalWeightManager) addSupportToForkedBranchVoters(voter Voter, forkedBranchVoters *BranchVoters, parentBranchIDs ledgerstate.BranchIDs, sequenceNumber uint64) (supportAdded bool) {
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

func (a *ApprovalWeightManager) voterSupportsAllBranches(voter Voter, branchIDs ledgerstate.BranchIDs) (allBranchesSupported bool) {
	allBranchesSupported = true
	for branchID := range branchIDs {
		if branchID == ledgerstate.MasterBranchID {
			continue
		}

		a.tangle.Storage.BranchVoters(branchID).Consume(func(branchVoters *BranchVoters) {
			allBranchesSupported = branchVoters.Has(voter)
		})

		if !allBranchesSupported {
			return false
		}
	}

	return allBranchesSupported
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApprovalWeightManagerEvents //////////////////////////////////////////////////////////////////////////////////

// ApprovalWeightManagerEvents represents events happening in the ApprovalWeightManager.
type ApprovalWeightManagerEvents struct {
	// MessageProcessed is triggered once a message is finished being processed by the ApprovalWeightManager.
	MessageProcessed *events.Event
	// BranchWeightChanged is triggered when a branch's weight changed.
	BranchWeightChanged *events.Event
	// MarkerWeightChanged is triggered when a marker's weight changed.
	MarkerWeightChanged *events.Event
}

// MarkerWeightChangedEvent holds information about a marker and its updated weight.
type MarkerWeightChangedEvent struct {
	Marker *markers.Marker
	Weight float64
}

// markerWeightChangedEventHandler is the caller function for events that hand over a MarkerWeightChangedEvent.
func markerWeightChangedEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*MarkerWeightChangedEvent))(params[0].(*MarkerWeightChangedEvent))
}

// BranchWeightChangedEvent holds information about a branch and its updated weight.
type BranchWeightChangedEvent struct {
	BranchID ledgerstate.BranchID
	Weight   float64
}

// branchWeightChangedEventHandler is the caller function for events that hand over a BranchWeightChangedEvent.
func branchWeightChangedEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*BranchWeightChangedEvent))(params[0].(*BranchWeightChangedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
