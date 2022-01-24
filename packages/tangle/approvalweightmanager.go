package tangle

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

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

// SupportersOfBranches returns the Supporters of the given BranchIDs.
func (a *ApprovalWeightManager) SupportersOfBranches(conflictBranchIDs ledgerstate.BranchIDs) (supporters *Supporters) {
	if len(conflictBranchIDs) == 0 {
		return NewSupporters()
	}

	for conflictBranchID := range conflictBranchIDs {
		if !a.tangle.Storage.BranchSupporters(conflictBranchID).Consume(func(branchSupporters *BranchSupporters) {
			if supporters == nil {
				supporters = branchSupporters.Supporters()
			} else {
				supporters = supporters.Intersect(branchSupporters.Supporters())
			}
		}) {
			return NewSupporters()
		}
	}

	return
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
	allParentsAdded = true

	for currentConflictBranchID := range conflictBranchIDs {
		currentVote := vote.WithBranchID(currentConflictBranchID)

		if a.differentVoteWithHigherSequenceExists(currentVote) {
			allParentsAdded = false
			continue
		}

		// Do not queue parents if a newer vote exists for this branch for this voter.
		if a.identicalVoteWithHigherSequenceExists(currentVote) {
			continue
		}

		a.tangle.LedgerState.Branch(currentConflictBranchID).ConsumeConflictBranch(func(conflictBranch *ledgerstate.ConflictBranch) {
			addedBranchesOfCurrentBranch, allParentsOfCurrentBranchAdded := a.determineBranchesToAdd(conflictBranch.Parents(), vote)
			allParentsAdded = allParentsAdded && allParentsOfCurrentBranchAdded

			addedBranches.AddAll(addedBranchesOfCurrentBranch)
		})

		if allParentsAdded {
			addedBranches.Add(currentConflictBranchID)
		}
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

		if _, exists := a.voteWithHigherSequence(currentVote); exists {
			continue
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

func (a *ApprovalWeightManager) differentVoteWithHigherSequenceExists(vote *Vote) (exists bool) {
	existingVote, exists := a.voteWithHigherSequence(vote)

	return exists && vote.Opinion != existingVote.Opinion
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

// region BranchWeight /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchWeight is a data structure that tracks the weight of a ledgerstate.BranchID.
type BranchWeight struct {
	branchID ledgerstate.BranchID
	weight   float64

	weightMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchWeight creates a new BranchWeight.
func NewBranchWeight(branchID ledgerstate.BranchID) (branchWeight *BranchWeight) {
	branchWeight = &BranchWeight{
		branchID: branchID,
	}

	branchWeight.Persist()
	branchWeight.SetModified()

	return
}

// BranchWeightFromBytes unmarshals a BranchWeight object from a sequence of bytes.
func BranchWeightFromBytes(bytes []byte) (branchWeight *BranchWeight, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchWeight, err = BranchWeightFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchWeight from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchWeightFromMarshalUtil unmarshals a BranchWeight object using a MarshalUtil (for easier unmarshaling).
func BranchWeightFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchWeight *BranchWeight, err error) {
	branchWeight = &BranchWeight{}
	if branchWeight.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	if branchWeight.weight, err = marshalUtil.ReadFloat64(); err != nil {
		err = errors.Errorf("failed to parse weight (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// BranchWeightFromObjectStorage restores a BranchWeight object from the object storage.
func BranchWeightFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = BranchWeightFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse BranchWeight from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the ledgerstate.BranchID that is being tracked.
func (b *BranchWeight) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchID
}

// Weight returns the weight of the ledgerstate.BranchID.
func (b *BranchWeight) Weight() (weight float64) {
	b.weightMutex.RLock()
	defer b.weightMutex.RUnlock()

	return b.weight
}

// SetWeight sets the weight for the ledgerstate.BranchID and returns true if it was modified.
func (b *BranchWeight) SetWeight(weight float64) (modified bool) {
	b.weightMutex.Lock()
	defer b.weightMutex.Unlock()

	if weight == b.weight {
		return false
	}

	b.weight = weight
	modified = true
	b.SetModified()

	return
}

// IncreaseWeight increases the weight for the ledgerstate.BranchID and returns the new weight.
func (b *BranchWeight) IncreaseWeight(weight float64) (newWeight float64) {
	b.weightMutex.Lock()
	defer b.weightMutex.Unlock()

	if weight != 0 {
		b.weight += weight
		b.SetModified()
	}

	return b.weight
}

// DecreaseWeight decreases the weight for the ledgerstate.BranchID and returns true if it was modified.
func (b *BranchWeight) DecreaseWeight(weight float64) (newWeight float64) {
	b.weightMutex.Lock()
	defer b.weightMutex.Unlock()

	if weight != 0 {
		b.weight -= weight
		b.SetModified()
	}

	return b.weight
}

// Bytes returns a marshaled version of the BranchWeight.
func (b *BranchWeight) Bytes() (marshaledBranchWeight []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human readable version of the BranchWeight.
func (b *BranchWeight) String() string {
	return stringify.Struct("BranchWeight",
		stringify.StructField("branchID", b.BranchID()),
		stringify.StructField("weight", b.Weight()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *BranchWeight) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchWeight) ObjectStorageKey() []byte {
	return b.BranchID().Bytes()
}

// ObjectStorageValue marshals the BranchWeight into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BranchWeight) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.Float64Size).
		WriteFloat64(b.Weight()).
		Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &BranchWeight{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBranchWeight ///////////////////////////////////////////////////////////////////////////////////////////

// CachedBranchWeight is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedBranchWeight struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedBranchWeight) Retain() *CachedBranchWeight {
	return &CachedBranchWeight{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedBranchWeight) Unwrap() *BranchWeight {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*BranchWeight)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranchWeight) Consume(consumer func(branchWeight *BranchWeight), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*BranchWeight))
	}, forceRelease...)
}

// String returns a human readable version of the CachedBranchWeight.
func (c *CachedBranchWeight) String() string {
	return stringify.Struct("CachedBranchWeight",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voter ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Voter is a type wrapper for identity.ID and defines a node that supports a branch or marker.
type Voter = identity.ID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Supporters ///////////////////////////////////////////////////////////////////////////////////////////////////

// Supporters is a set of node identities that votes for a particular Branch.
type Supporters struct {
	set.Set
}

// NewSupporters is the constructor of the Supporters type.
func NewSupporters() (supporters *Supporters) {
	return &Supporters{
		Set: set.New(),
	}
}

// Add adds a new Supporter to the Set and returns true if the Supporter was not present in the set before.
func (s *Supporters) Add(supporter Voter) (added bool) {
	return s.Set.Add(supporter)
}

// Delete removes the Supporter from the Set and returns true if it did exist.
func (s *Supporters) Delete(supporter Voter) (deleted bool) {
	return s.Set.Delete(supporter)
}

// Has returns true if the Supporter exists in the Set.
func (s *Supporters) Has(supporter Voter) (has bool) {
	return s.Set.Has(supporter)
}

// ForEach iterates through the Supporters and calls the callback for every element.
func (s *Supporters) ForEach(callback func(supporter Voter)) {
	s.Set.ForEach(func(element interface{}) {
		callback(element.(Voter))
	})
}

// Clone returns a copy of the Supporters.
func (s *Supporters) Clone() (clonedSupporters *Supporters) {
	clonedSupporters = NewSupporters()
	s.ForEach(func(supporter Voter) {
		clonedSupporters.Add(supporter)
	})

	return
}

// Intersect creates an intersection of two set of Supporters.
func (s *Supporters) Intersect(other *Supporters) (intersection *Supporters) {
	intersection = NewSupporters()
	s.ForEach(func(supporter Voter) {
		if other.Has(supporter) {
			intersection.Add(supporter)
		}
	})

	return
}

// String returns a human readable version of the Supporters.
func (s *Supporters) String() string {
	structBuilder := stringify.StructBuilder("Supporters")
	s.ForEach(func(supporter Voter) {
		structBuilder.AddField(stringify.StructField(supporter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchSupporters /////////////////////////////////////////////////////////////////////////////////////////////

// BranchSupporters is a data structure that tracks which nodes support a branch.
type BranchSupporters struct {
	branchID   ledgerstate.BranchID
	supporters *Supporters

	supportersMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchSupporters is the constructor for the BranchSupporters object.
func NewBranchSupporters(branchID ledgerstate.BranchID) (branchSupporters *BranchSupporters) {
	branchSupporters = &BranchSupporters{
		branchID:   branchID,
		supporters: NewSupporters(),
	}

	branchSupporters.Persist()
	branchSupporters.SetModified()

	return
}

// BranchSupportersFromBytes unmarshals a BranchSupporters object from a sequence of bytes.
func BranchSupportersFromBytes(bytes []byte) (branchSupporters *BranchSupporters, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchSupporters, err = BranchSupportersFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceSupporters from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchSupportersFromMarshalUtil unmarshals a BranchSupporters object using a MarshalUtil (for easier unmarshaling).
func BranchSupportersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchSupporters *BranchSupporters, err error) {
	branchSupporters = &BranchSupporters{}
	if branchSupporters.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	supportersCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse supporters count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	branchSupporters.supporters = NewSupporters()
	for i := uint64(0); i < supportersCount; i++ {
		supporter, supporterErr := identity.IDFromMarshalUtil(marshalUtil)
		if supporterErr != nil {
			err = errors.Errorf("failed to parse Supporter (%v): %w", supporterErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchSupporters.supporters.Add(supporter)
	}

	return
}

// BranchSupportersFromObjectStorage restores a BranchSupporters object from the object storage.
func BranchSupportersFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = BranchSupportersFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse BranchSupporters from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the ledgerstate.BranchID that is being tracked.
func (b *BranchSupporters) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchID
}

// Has returns true if the given Voter is currently supporting this Branch.
func (b *BranchSupporters) Has(voter Voter) bool {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	return b.supporters.Has(voter)
}

// AddSupporter adds a new Supporter to the tracked ledgerstate.BranchID.
func (b *BranchSupporters) AddSupporter(supporter Voter) (added bool) {
	b.supportersMutex.Lock()
	defer b.supportersMutex.Unlock()

	if added = b.supporters.Add(supporter); !added {
		return
	}
	b.SetModified()

	return
}

// AddSupporters adds the supporters set to the tracked ledgerstate.BranchID.
func (b *BranchSupporters) AddSupporters(supporters *Supporters) (added bool) {
	supporters.ForEach(func(supporter Voter) {
		if b.supporters.Add(supporter) {
			added = true
		}
	})

	if added {
		b.SetModified()
	}

	return
}

// DeleteSupporter deletes a Supporter from the tracked ledgerstate.BranchID.
func (b *BranchSupporters) DeleteSupporter(supporter Voter) (deleted bool) {
	b.supportersMutex.Lock()
	defer b.supportersMutex.Unlock()

	if deleted = b.supporters.Delete(supporter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Supporters returns the set of Supporters that are supporting the given ledgerstate.BranchID.
func (b *BranchSupporters) Supporters() (supporters *Supporters) {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	return b.supporters.Clone()
}

// Bytes returns a marshaled version of the BranchSupporters.
func (b *BranchSupporters) Bytes() (marshaledBranchSupporters []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human readable version of the BranchSupporters.
func (b *BranchSupporters) String() string {
	return stringify.Struct("BranchSupporters",
		stringify.StructField("branchID", b.BranchID()),
		stringify.StructField("supporters", b.Supporters()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *BranchSupporters) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchSupporters) ObjectStorageKey() []byte {
	return b.BranchID().Bytes()
}

// ObjectStorageValue marshals the BranchSupporters into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BranchSupporters) ObjectStorageValue() []byte {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	marshalUtil := marshalutil.New(marshalutil.Uint64Size + b.supporters.Size()*identity.IDLength)
	marshalUtil.WriteUint64(uint64(b.supporters.Size()))

	b.supporters.ForEach(func(supporter Voter) {
		marshalUtil.WriteBytes(supporter.Bytes())
	})

	return marshalUtil.Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &BranchSupporters{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBranchSupporters ///////////////////////////////////////////////////////////////////////////////////////

// CachedBranchSupporters is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedBranchSupporters struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedBranchSupporters) Retain() *CachedBranchSupporters {
	return &CachedBranchSupporters{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedBranchSupporters) Unwrap() *BranchSupporters {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*BranchSupporters)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranchSupporters) Consume(consumer func(branchSupporters *BranchSupporters), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*BranchSupporters))
	}, forceRelease...)
}

// String returns a human readable version of the CachedBranchSupporters.
func (c *CachedBranchSupporters) String() string {
	return stringify.Struct("CachedBranchSupporters",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion is a type that represents the Opinion of a node on a certain Branch.
type Opinion uint8

const (
	// UndefinedOpinion represents the zero value of the Opinion type.
	UndefinedOpinion Opinion = iota

	// Confirmed represents the Opinion that a given Branch is the winning one.
	Confirmed

	// Rejected represents the Opinion that a given Branch is the loosing one.
	Rejected
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestMarkerVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestMarkerVotesKeyPartition defines the partition of the storage key of the LastMarkerVotes model.
var LatestMarkerVotesKeyPartition = objectstorage.PartitionKey(markers.SequenceIDLength, identity.IDLength)

// LatestMarkerVotes represents the markers supported from a certain Voter.
type LatestMarkerVotes struct {
	sequenceID  markers.SequenceID
	voter       Voter
	latestVotes *thresholdmap.ThresholdMap

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(sequenceID markers.SequenceID, voter Voter) (newLatestMarkerVotes *LatestMarkerVotes) {
	newLatestMarkerVotes = &LatestMarkerVotes{
		sequenceID:  sequenceID,
		voter:       voter,
		latestVotes: thresholdmap.New(thresholdmap.UpperThresholdMode, markers.IndexComparator),
	}

	newLatestMarkerVotes.SetModified()
	newLatestMarkerVotes.Persist()

	return
}

func LatestMarkerVotesFromBytes(bytes []byte) (latestMarkerVotes *LatestMarkerVotes, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestMarkerVotes, err = LatestMarkerVotesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestVotes from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func LatestMarkerVotesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestMarkerVotes *LatestMarkerVotes, err error) {
	latestMarkerVotes = &LatestMarkerVotes{}
	if latestMarkerVotes.sequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
	}
	if latestMarkerVotes.voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	mapSize, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, errors.Errorf("failed to read mapSize from MarshalUtil: %w", err)
	}

	latestMarkerVotes.latestVotes = thresholdmap.New(thresholdmap.UpperThresholdMode, markers.IndexComparator)
	for i := uint64(0); i < mapSize; i++ {
		markerIndex, markerIndexErr := markers.IndexFromMarshalUtil(marshalUtil)
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read Index from MarshalUtil: %w", markerIndexErr)
		}

		sequenceNumber, sequenceNumberErr := marshalUtil.ReadUint64()
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read sequence number from MarshalUtil: %w", sequenceNumberErr)
		}

		latestMarkerVotes.latestVotes.Set(markerIndex, sequenceNumber)
	}

	return latestMarkerVotes, nil
}

func LatestMarkerVotesFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = LatestMarkerVotesFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes from bytes: %w", err)
		return
	}

	return
}

func (l *LatestMarkerVotes) Voter() Voter {
	return l.voter
}

func (l *LatestMarkerVotes) SequenceNumber(index markers.Index) (sequenceNumber uint64, exists bool) {
	l.RLock()
	defer l.RUnlock()

	key, exists := l.latestVotes.Get(index)
	if !exists {
		return 0, exists
	}

	return key.(uint64), exists
}

func (l *LatestMarkerVotes) Store(index markers.Index, sequenceNumber uint64) (stored bool, previousHighestIndex markers.Index) {
	l.Lock()
	defer l.Unlock()

	if maxElement := l.latestVotes.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key().(markers.Index)
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.latestVotes.Ceiling(index)
	if ceilingExists && sequenceNumber < ceilingValue.(uint64) {
		return false, previousHighestIndex
	}

	// set the new value
	l.latestVotes.Set(index, sequenceNumber)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.latestVotes.Floor(index - 1)
	for floorExists && floorValue.(uint64) < sequenceNumber {
		l.latestVotes.Delete(floorKey)

		floorKey, floorValue, floorExists = l.latestVotes.Floor(index - 1)
	}

	l.SetModified()

	return true, previousHighestIndex
}

func (l *LatestMarkerVotes) String() string {
	builder := stringify.StructBuilder("LatestMarkerVotes")

	l.latestVotes.ForEach(func(node *thresholdmap.Element) bool {
		builder.AddField(stringify.StructField(node.Key().(markers.Index).String(), node.Value()))

		return true
	})

	return builder.String()
}

func (l *LatestMarkerVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}
func (l *LatestMarkerVotes) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (l *LatestMarkerVotes) ObjectStorageKey() []byte {
	return marshalutil.New().
		Write(l.sequenceID).
		Write(l.voter).
		Bytes()
}

func (l *LatestMarkerVotes) ObjectStorageValue() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(l.latestVotes.Size()))
	l.latestVotes.ForEach(func(node *thresholdmap.Element) bool {
		marshalUtil.Write(node.Key().(markers.Index))
		marshalUtil.WriteUint64(node.Value().(uint64))

		return true
	})

	return marshalUtil.Bytes()
}

var _ objectstorage.StorableObject = &LatestMarkerVotes{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestMarkerVotes //////////////////////////////////////////////////////////////////////////////////////

// CachedLatestMarkerVotes is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedLatestMarkerVotes struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedLatestMarkerVotes) Retain() *CachedLatestMarkerVotes {
	return &CachedLatestMarkerVotes{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedLatestMarkerVotes) Unwrap() *LatestMarkerVotes {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*LatestMarkerVotes)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedLatestMarkerVotes) Consume(consumer func(latestMarkerVotes *LatestMarkerVotes), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*LatestMarkerVotes))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedLatestMarkerVotes.
func (c *CachedLatestMarkerVotes) String() string {
	return stringify.Struct("CachedLatestMarkerVotes",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestMarkerVotesByVoter ///////////////////////////////////////////////////////////////////////////////

type CachedLatestMarkerVotesByVoter map[Voter]*CachedLatestMarkerVotes

func (c CachedLatestMarkerVotesByVoter) Consume(consumer func(latestMarkerVotes *LatestMarkerVotes), forceRelease ...bool) (consumed bool) {
	for _, cachedLatestMarkerVotes := range c {
		consumed = cachedLatestMarkerVotes.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestBranchVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestBranchVotes represents the branch supported from an Issuer
type LatestBranchVotes struct {
	voter       Voter
	latestVotes map[ledgerstate.BranchID]*Vote

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

func (l *LatestBranchVotes) Vote(branchID ledgerstate.BranchID) (vote *Vote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.latestVotes[branchID]

	return
}

func (l *LatestBranchVotes) Store(vote *Vote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.latestVotes[vote.BranchID]; exists && currentVote.SequenceNumber >= vote.SequenceNumber {
		return false
	}

	l.latestVotes[vote.BranchID] = vote
	l.SetModified()

	return true
}

// NewLatestBranchVotes creates a new LatestVotes.
func NewLatestBranchVotes(supporter Voter) (latestVotes *LatestBranchVotes) {
	latestVotes = &LatestBranchVotes{
		voter:       supporter,
		latestVotes: make(map[ledgerstate.BranchID]*Vote),
	}

	latestVotes.Persist()
	latestVotes.SetModified()

	return
}

// LatestVotesFromBytes unmarshals a LatestVotes object from a sequence of bytes.
func LatestVotesFromBytes(bytes []byte) (latestVotes *LatestBranchVotes, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestVotes, err = LatestVotesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestVotes from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// LatestVotesFromMarshalUtil unmarshals a LatestVotes object using a MarshalUtil (for easier unmarshalling).
func LatestVotesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestVotes *LatestBranchVotes, err error) {
	latestVotes = &LatestBranchVotes{}
	if latestVotes.voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
		return
	}

	mapSize, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse map size (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	latestVotes.latestVotes = make(map[ledgerstate.BranchID]*Vote, int(mapSize))

	for i := uint64(0); i < mapSize; i++ {
		branchID, voteErr := ledgerstate.BranchIDFromMarshalUtil(marshalUtil)
		if voteErr != nil {
			err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", voteErr)
			return
		}

		vote, voteErr := VoteFromMarshalUtil(marshalUtil)
		if voteErr != nil {
			err = errors.Errorf("failed to parse Vote from MarshalUtil: %w", voteErr)
			return
		}

		latestVotes.latestVotes[branchID] = vote
	}

	return
}

// LatestVotesFromObjectStorage restores a LatestVotes object from the object storage.
func LatestVotesFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = LatestVotesFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse LatestVotes from bytes: %w", err)
		return
	}

	return
}

// Bytes returns a marshaled version of the LatestVotes.
func (l *LatestBranchVotes) Bytes() (marshaledSequenceSupporters []byte) {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// String returns a human-readable version of the LatestVotes.
func (l *LatestBranchVotes) String() string {
	return stringify.Struct("LatestVotes",
		stringify.StructField("voter", l.voter),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (l *LatestBranchVotes) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (l *LatestBranchVotes) ObjectStorageKey() []byte {
	return l.voter.Bytes()
}

// ObjectStorageValue marshals the LatestVotes into a sequence of bytes that are used as the value part in the
// object storage.
func (l *LatestBranchVotes) ObjectStorageValue() []byte {
	l.RLock()
	defer l.RUnlock()

	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(len(l.latestVotes)))

	for branchID, vote := range l.latestVotes {
		marshalUtil.Write(branchID)
		marshalUtil.Write(vote)
	}

	return marshalUtil.Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &LatestBranchVotes{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestBranchVotes //////////////////////////////////////////////////////////////////////////////////////

// CachedLatestBranchVotes is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedLatestBranchVotes struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedLatestBranchVotes) Retain() *CachedLatestBranchVotes {
	return &CachedLatestBranchVotes{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedLatestBranchVotes) Unwrap() *LatestBranchVotes {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*LatestBranchVotes)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedLatestBranchVotes) Consume(consumer func(latestVotes *LatestBranchVotes), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*LatestBranchVotes))
	}, forceRelease...)
}

// String returns a human readable version of the CachedLatestBranchVotes.
func (c *CachedLatestBranchVotes) String() string {
	return stringify.Struct("CachedLatestBranchVotes",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// Vote represents a struct that holds information about the shared Opinion of a node regarding an individual Branch.
type Vote struct {
	Voter          Voter
	BranchID       ledgerstate.BranchID
	Opinion        Opinion
	SequenceNumber uint64
}

// VoteFromMarshalUtil unmarshals a Vote structure using a MarshalUtil (for easier unmarshalling).
func VoteFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (vote *Vote, err error) {
	vote = &Vote{}

	if vote.Voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	if vote.BranchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
	}

	untypedOpinion, err := marshalUtil.ReadUint8()
	if err != nil {
		return nil, errors.Errorf("failed to parse Opinion from MarshalUtil: %w", err)
	}
	vote.Opinion = Opinion(untypedOpinion)

	if vote.SequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		return nil, errors.Errorf("failed to parse SequenceNumber from MarshalUtil: %w", err)
	}

	return
}

// WithOpinion derives a vote for the given Opinion.
func (v *Vote) WithOpinion(opinion Opinion) (voteWithOpinion *Vote) {
	return &Vote{
		Voter:          v.Voter,
		BranchID:       v.BranchID,
		Opinion:        opinion,
		SequenceNumber: v.SequenceNumber,
	}
}

// WithBranchID derives a vote for the given BranchID.
func (v *Vote) WithBranchID(branchID ledgerstate.BranchID) (rejectedVote *Vote) {
	return &Vote{
		Voter:          v.Voter,
		BranchID:       branchID,
		Opinion:        v.Opinion,
		SequenceNumber: v.SequenceNumber,
	}
}

// Bytes returns the bytes of the Vote
func (v *Vote) Bytes() []byte {
	return marshalutil.New().
		Write(v.Voter).
		Write(v.BranchID).
		WriteUint8(uint8(v.Opinion)).
		WriteUint64(v.SequenceNumber).
		Bytes()
}

func (v *Vote) String() string {
	return stringify.Struct("Vote",
		stringify.StructField("Voter", v.Voter),
		stringify.StructField("BranchID", v.BranchID),
		stringify.StructField("Opinion", int(v.Opinion)),
		stringify.StructField("SequenceNumber", v.SequenceNumber),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
