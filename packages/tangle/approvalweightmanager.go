package tangle

import (
	"math"
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

const (
	minSupporterWeight float64 = 0.000000000000001
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
// supporters of the Message's ledgerstate.Branch and approved markers.Marker and eventually triggers events when
// approval weights for branch and markers are reached.
func (a *ApprovalWeightManager) processBookedMessage(messageID MessageID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.updateBranchSupporters(message)
		a.updateSequenceSupporters(message)

		a.Events.MessageProcessed.Trigger(messageID)
	})
}

// WeightOfBranch returns the weight of the given Branch that was added by Supporters of the given epoch.
func (a *ApprovalWeightManager) WeightOfBranch(branchID ledgerstate.BranchID) (weight float64) {
	conflictBranchIDs, err := a.tangle.LedgerState.BranchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	weight = math.MaxFloat64
	for conflictBranchID := range conflictBranchIDs {
		a.tangle.Storage.BranchWeight(conflictBranchID).Consume(func(branchWeight *BranchWeight) {
			if branchWeight.Weight() <= weight {
				weight = branchWeight.Weight()
			}
		})
	}

	// We don't have any information stored about this branch, thus we default to weight=0.
	if weight == math.MaxFloat64 {
		return 0
	}
	return
}

// WeightOfMarker returns the weight of the given marker based on the anchorTime.
func (a *ApprovalWeightManager) WeightOfMarker(marker *markers.Marker, anchorTime time.Time) (weight float64) {
	activeWeight, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantSupporters()

	supporterWeight := float64(0)
	for supporter := range a.markerVotes(marker) {
		supporterWeight += activeWeight[supporter]
	}

	return supporterWeight / totalWeight
}

// Shutdown shuts down the ApprovalWeightManager and persists its state.
func (a *ApprovalWeightManager) Shutdown() {}

func (a *ApprovalWeightManager) isRelevantSupporter(message *Message) bool {
	supporterWeight, totalWeight := a.tangle.WeightProvider.Weight(message)

	return supporterWeight/totalWeight >= minSupporterWeight
}

// SupportersOfConflictBranch returns the Supporters of the given conflictbranch ledgerstate.BranchID.
func (a *ApprovalWeightManager) SupportersOfConflictBranch(branchID ledgerstate.BranchID) (supporters *Supporters) {
	if !a.tangle.Storage.BranchSupporters(branchID).Consume(func(branchSupporters *BranchSupporters) {
		supporters = branchSupporters.Supporters()
	}) {
		supporters = NewSupporters()
	}
	return
}

// markerVotes returns a map containing Voters associated to their respective SequenceNumbers.
func (a *ApprovalWeightManager) markerVotes(marker *markers.Marker) (markerVotes map[Voter]uint64) {
	markerVotes = make(map[Voter]uint64)
	a.tangle.Storage.AllLatestMarkerVotes(marker.SequenceID()).Consume(func(latestMarkerVotes *LatestMarkerVotes) {
		lastSequenceNumber, exists := latestMarkerVotes.SequenceNumber(marker.Index())
		if !exists {
			return
		}

		markerVotes[latestMarkerVotes.Voter()] = lastSequenceNumber
	})

	return markerVotes
}

func (a *ApprovalWeightManager) updateBranchSupporters(message *Message) {
	branchesOfMessage, err := a.tangle.Booker.MessageBranchIDs(message.ID())
	if err != nil {
		panic(err)
	}

	voter := identity.NewID(message.IssuerPublicKey())
	vote := &Vote{
		Voter:          voter,
		SequenceNumber: message.SequenceNumber(),
	}

	addedBranchIDs, revokedBranchIDs, isInvalid := a.determineVotes(branchesOfMessage, vote)
	if isInvalid {
		a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetSubjectivelyInvalid(true)
		})
	}

	if !a.isRelevantSupporter(message) {
		return
	}

	a.tangle.Storage.LatestVotes(voter, NewLatestBranchVotes).Consume(func(latestVotes *LatestBranchVotes) {
		addedVote := vote.WithOpinion(Confirmed)
		for addBranchID := range addedBranchIDs {
			latestVotes.Store(addedVote.WithBranchID(addBranchID))
			a.addSupportToBranch(addBranchID, voter)
		}

		revokedVote := vote.WithOpinion(Rejected)
		for revokedBranchID := range revokedBranchIDs {
			latestVotes.Store(revokedVote.WithBranchID(revokedBranchID))
			a.revokeSupportFromBranch(revokedBranchID, voter)
		}
	})
}

func (a *ApprovalWeightManager) determineVotes(votedBranchIDs ledgerstate.BranchIDs, vote *Vote) (addedBranches, revokedBranches ledgerstate.BranchIDs, isInvalid bool) {
	addedBranches = ledgerstate.NewBranchIDs()
	for votedBranchID := range votedBranchIDs {
		conflictingBranchWithHigherVoteExists := false
		a.tangle.LedgerState.ForEachConflictingBranchID(votedBranchID, func(conflictingBranchID ledgerstate.BranchID) bool {
			conflictingBranchWithHigherVoteExists = a.identicalVoteWithHigherSequenceExists(vote.WithBranchID(conflictingBranchID).WithOpinion(Confirmed))

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

// determineBranchesToAdd iterates through the past cone of the given ConflictBranches and determines the BranchIDs that
// are affected by the Vote.
func (a *ApprovalWeightManager) determineBranchesToAdd(conflictBranchIDs ledgerstate.BranchIDs, vote *Vote) (addedBranches ledgerstate.BranchIDs, allParentsAdded bool) {
	addedBranches = ledgerstate.NewBranchIDs()

	for currentConflictBranchID := range conflictBranchIDs {
		currentVote := vote.WithBranchID(currentConflictBranchID)

		// Do not queue parents if a newer vote exists for this branch for this voter.
		if a.identicalVoteWithHigherSequenceExists(currentVote) {
			continue
		}

		a.tangle.LedgerState.Branch(currentConflictBranchID).ConsumeConflictBranch(func(conflictBranch *ledgerstate.ConflictBranch) {
			addedBranchesOfCurrentBranch, allParentsOfCurrentBranchAdded := a.determineBranchesToAdd(conflictBranch.Parents(), vote)
			allParentsAdded = allParentsAdded && allParentsOfCurrentBranchAdded

			addedBranches.AddAll(addedBranchesOfCurrentBranch)
		})

		addedBranches.Add(currentConflictBranchID)
	}

	return
}

// determineBranchesToRevoke determines which Branches of the conflicting future cone of the added Branches are affected
// by the vote and if the vote is valid (not voting for conflicting Branches).
func (a *ApprovalWeightManager) determineBranchesToRevoke(addedBranches, votedBranches ledgerstate.BranchIDs, vote *Vote) (revokedBranches ledgerstate.BranchIDs, isInvalid bool) {
	revokedBranches = ledgerstate.NewBranchIDs()
	subTractionWalker := walker.New()
	for addedBranch := range addedBranches {
		a.tangle.LedgerState.ForEachConflictingBranchID(addedBranch, func(conflictingBranchID ledgerstate.BranchID) bool {
			subTractionWalker.Push(conflictingBranchID)

			return true
		})
	}

	for subTractionWalker.HasNext() {
		currentVote := vote.WithBranchID(subTractionWalker.Next().(ledgerstate.BranchID))

		if isInvalid = addedBranches.Contains(currentVote.BranchID) || votedBranches.Contains(currentVote.BranchID); isInvalid {
			return
		}

		revokedBranches.Add(currentVote.BranchID)

		a.tangle.LedgerState.ChildBranches(currentVote.BranchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
			if childBranch.ChildBranchType() == ledgerstate.AggregatedBranchType {
				return
			}

			subTractionWalker.Push(childBranch.ChildBranchID())
		})
	}

	return
}

func (a *ApprovalWeightManager) identicalVoteWithHigherSequenceExists(vote *Vote) (exists bool) {
	existingVote, exists := a.voteWithHigherSequence(vote)

	return exists && vote.Opinion == existingVote.Opinion
}

func (a *ApprovalWeightManager) voteWithHigherSequence(vote *Vote) (existingVote *Vote, exists bool) {
	a.tangle.Storage.LatestVotes(vote.Voter).Consume(func(latestVotes *LatestBranchVotes) {
		existingVote, exists = latestVotes.Vote(vote.BranchID)
	})

	return existingVote, exists && existingVote.SequenceNumber > vote.SequenceNumber
}

func (a *ApprovalWeightManager) addSupportToBranch(branchID ledgerstate.BranchID, voter Voter) {
	if branchID == ledgerstate.MasterBranchID {
		return
	}

	a.tangle.Storage.BranchSupporters(branchID, NewBranchSupporters).Consume(func(branchSupporters *BranchSupporters) {
		branchSupporters.AddSupporter(voter)
	})

	a.updateBranchWeight(branchID)
}

func (a *ApprovalWeightManager) revokeSupportFromBranch(branchID ledgerstate.BranchID, voter Voter) {
	a.tangle.Storage.BranchSupporters(branchID, NewBranchSupporters).Consume(func(branchSupporters *BranchSupporters) {
		branchSupporters.DeleteSupporter(voter)
	})

	a.updateBranchWeight(branchID)
}

func (a *ApprovalWeightManager) updateSequenceSupporters(message *Message) {
	if !a.isRelevantSupporter(message) {
		return
	}

	a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		// Do not revisit markers that have already been visited. With the like switch there can be cycles in the sequence DAG
		// which results in endless walks.
		supportWalker := walker.New(false)

		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			if sequenceID == 0 {
				return true
			}

			supportWalker.Push(*markers.NewMarker(sequenceID, index))

			return true
		})

		for supportWalker.HasNext() {
			a.addSupportToMarker(supportWalker.Next().(markers.Marker), message, supportWalker)
		}
	})
}

func (a *ApprovalWeightManager) addSupportToMarker(marker markers.Marker, message *Message, walk *walker.Walker) {
	// We don't add the supporter and abort if the marker is already confirmed. This prevents walking too much in the sequence DAG.
	// However, it might lead to inaccuracies when creating a new branch once a conflict arrives and we copy over the
	// supporters of the marker to the branch. Since the marker is already seen as confirmed it should not matter too much though.
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
				if sequenceID == 0 {
					return true
				}

				walk.Push(*markers.NewMarker(sequenceID, index))

				return true
			})
		})
	})
}

// updateMarkerWeights updates the marker weights in the given range and triggers the MarkerWeightChanged event.
func (a *ApprovalWeightManager) updateMarkerWeights(sequenceID markers.SequenceID, rangeStartIndex markers.Index, rangeEndIndex markers.Index) {
	if rangeStartIndex <= 1 {
		rangeStartIndex = a.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(sequenceID)
	}

	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantSupporters()
	for i := rangeStartIndex; i <= rangeEndIndex; i++ {
		currentMarker := markers.NewMarker(sequenceID, i)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if a.tangle.Booker.MarkersManager.MessageID(currentMarker) == EmptyMessageID {
			continue
		}

		supporterWeight := float64(0)
		for supporter := range a.markerVotes(currentMarker) {
			supporterWeight += activeWeights[supporter]
		}

		a.Events.MarkerWeightChanged.Trigger(&MarkerWeightChangedEvent{currentMarker, supporterWeight / totalWeight})
	}
}

func (a *ApprovalWeightManager) updateBranchWeight(branchID ledgerstate.BranchID) {
	activeWeights, totalWeight := a.tangle.WeightProvider.WeightsOfRelevantSupporters()

	var supporterWeight float64
	a.SupportersOfConflictBranch(branchID).ForEach(func(supporter Voter) {
		supporterWeight += activeWeights[supporter]
	})

	newBranchWeight := supporterWeight / totalWeight

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
		a.tangle.Storage.BranchSupporters(forkedBranchID, NewBranchSupporters).Consume(func(forkedBranchSupporters *BranchSupporters) {
			a.tangle.LedgerState.Branch(forkedBranchID).Consume(func(forkedBranch ledgerstate.Branch) {
				if !a.addSupportToForkedBranchSupporters(identity.NewID(message.IssuerPublicKey()), forkedBranchSupporters, forkedBranch.(*ledgerstate.ConflictBranch).Parents(), message.SequenceNumber()) {
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
	a.tangle.Storage.BranchSupporters(forkedBranchID, NewBranchSupporters).Consume(func(branchSupporters *BranchSupporters) {
		a.tangle.LedgerState.Branch(forkedBranchID).Consume(func(forkedBranch ledgerstate.Branch) {
			parentBranchIDs := forkedBranch.(*ledgerstate.ConflictBranch).Parents()

			for voter, sequenceNumber := range a.markerVotes(marker) {
				if !a.addSupportToForkedBranchSupporters(voter, branchSupporters, parentBranchIDs, sequenceNumber) {
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

func (a *ApprovalWeightManager) addSupportToForkedBranchSupporters(voter Voter, forkedBranchSupporters *BranchSupporters, parentBranchIDs ledgerstate.BranchIDs, sequenceNumber uint64) (supportAdded bool) {
	if !a.voterSupportsAllBranches(voter, parentBranchIDs) {
		return false
	}

	a.tangle.Storage.LatestVotes(voter, NewLatestBranchVotes).Consume(func(latestVotes *LatestBranchVotes) {
		supportAdded = latestVotes.Store(&Vote{
			Voter:          voter,
			BranchID:       forkedBranchSupporters.BranchID(),
			Opinion:        Confirmed,
			SequenceNumber: sequenceNumber,
		})
	})

	return supportAdded && forkedBranchSupporters.AddSupporter(voter)
}

func (a *ApprovalWeightManager) voterSupportsAllBranches(voter Voter, branchIDs ledgerstate.BranchIDs) (allBranchesSupported bool) {
	allBranchesSupported = true
	for branchID := range branchIDs {
		if branchID == ledgerstate.MasterBranchID {
			continue
		}

		a.tangle.Storage.BranchSupporters(branchID).Consume(func(branchSupporters *BranchSupporters) {
			allBranchesSupported = branchSupporters.Has(voter)
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
