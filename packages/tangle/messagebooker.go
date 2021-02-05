package tangle

import (
	"container/list"
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// MessageBooker is a manager that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type MessageBooker struct {
	newMarkerIndexStrategy markers.IncreaseIndexCallback
	messageStore           *MessageStore
	markersManager         *markers.Manager
	branchDAG              *ledgerstate.BranchDAG
	utxoDAG                *ledgerstate.UTXODAG
}

// NewMessageBooker is the constructor of a MessageBooker.
func NewMessageBooker() (messageBooker *MessageBooker) {
	messageBooker = &MessageBooker{}

	return
}

func (m *MessageBooker) Book(cachedMessage *CachedMessage, cachedMessageMetadata *CachedMessageMetadata) (err error) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	message := cachedMessage.Unwrap()
	if message == nil {
		panic("failed to unwrap Message")
	}
	messageMetadata := cachedMessageMetadata.Unwrap()
	if messageMetadata == nil {
		panic(fmt.Sprintf("failed to unwrap MessageMetadata with %s", cachedMessageMetadata.ID()))
	}

	if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
		m.bookMessageContainingTransaction(message, messageMetadata, payload.(*ledgerstate.Transaction))
	} else {
		if err = m.bookMessageContainingData(message, messageMetadata); err != nil {
			err = xerrors.Errorf("failed to book Message containing Data: %w", err)
			return
		}
	}

	// Trigger Events

	return
}

// bookMessageContainingData is an internal utility function that books a Message containing arbitrary data.
func (m *MessageBooker) bookMessageContainingData(message *Message, messageMetadata *MessageMetadata) (err error) {
	targetBranch, err := m.determineTargetBranch(m.branchIDsOfStrongParents(message))
	if err != nil {
		err = xerrors.Errorf("failed to determine target Branch when booking Message with %s: %w", message.ID(), err)
		return
	}

	messageMetadata.SetBranchID(targetBranch)
	messageMetadata.SetStructureDetails(m.inheritStructureDetails(message, targetBranch, m.structuredDetailsOfStrongParents(message)))
	messageMetadata.SetBooked(true)

	return
}

// TODO: ADD LAZY BOOKING STUFF

// bookMessageContainingTransaction is an internal utility function that books a Message containing a Transaction.
func (m *MessageBooker) bookMessageContainingTransaction(message *Message, messageMetadata *MessageMetadata, transaction *ledgerstate.Transaction) {
	message.Parents()

	for referencedTransactionID := range m.referencedTransactionIDs(transaction) {
		for _, referencedMessageID := range m.Attachments(referencedTransactionID) {
			m.messageStore.MessageMetadata(referencedMessageID).Consume(func(messageMetadata *MessageMetadata) {

			})
		}
	}

	// store attachment

	//m.markersManager.IsInPastCone()

	// past cone check
	consumedTransactions := make(map[ledgerstate.TransactionID]types.Empty)
	for _, input := range transaction.Essence().Inputs() {
		consumedTransactions[input.(*ledgerstate.UTXOInput).ReferencedOutputID().TransactionID()] = types.Void
	}
}

func (m *MessageBooker) IsInPastCone(earlierMessageID MessageID, laterMessageID MessageID) bool {
	if m.IsInStrongPastCone(earlierMessageID, laterMessageID) {
		return true
	}

	cachedWeakApprovers := m.messageStore.Approvers(earlierMessageID, WeakApprover)
	defer cachedWeakApprovers.Release()

	for _, weakApprover := range cachedWeakApprovers.Unwrap() {
		if weakApprover == nil {
			continue
		}

		if m.IsInStrongPastCone(weakApprover.ApproverMessageID(), laterMessageID) {
			return true
		}
	}

	return false
}

func (m *MessageBooker) IsInStrongPastCone(earlierMessageID MessageID, laterMessageID MessageID) bool {
	// return early if the MessageIDs are the same
	if earlierMessageID == laterMessageID {
		return true
	}

	// retrieve the StructureDetails of both messages
	var earlierMessageStructureDetails, laterMessageStructureDetails *markers.StructureDetails
	m.messageStore.MessageMetadata(earlierMessageID).Consume(func(messageMetadata *MessageMetadata) {
		earlierMessageStructureDetails = messageMetadata.StructureDetails()
	})
	m.messageStore.MessageMetadata(laterMessageID).Consume(func(messageMetadata *MessageMetadata) {
		laterMessageStructureDetails = messageMetadata.StructureDetails()
	})

	// return false if one of the StructureDetails could not be retrieved (one of the messages was removed already / not
	// booked yet)
	if earlierMessageStructureDetails == nil || laterMessageStructureDetails == nil {
		return false
	}

	// perform past cone check
	switch m.markersManager.IsInPastCone(earlierMessageStructureDetails, laterMessageStructureDetails) {
	case types.True:
		return true
	case types.False:
		return false
	default:
		return m.isInStrongPastConeWalk(earlierMessageID, laterMessageID)
	}
}

func (m *MessageBooker) isInStrongPastConeWalk(earlierMessageID MessageID, laterMessageID MessageID) bool {
	stack := list.New()
	m.messageStore.Approvers(earlierMessageID, StrongApprover).Consume(func(approver *Approver) {
		stack.PushBack(approver.ApproverMessageID())
	})

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		currentMessageID := firstElement.Value.(MessageID)
		if currentMessageID == laterMessageID {
			return true
		}

		m.messageStore.MessageMetadata(currentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			if structureDetails := messageMetadata.StructureDetails(); structureDetails != nil {
				if !structureDetails.IsPastMarker {
					m.messageStore.Approvers(earlierMessageID, StrongApprover).Consume(func(approver *Approver) {
						stack.PushBack(approver.ApproverMessageID())
					})
				}
			}
		})
	}

	return false
}

func (m *MessageBooker) referencedTransactionIDs(transaction *ledgerstate.Transaction) (transactionIDs map[ledgerstate.TransactionID]types.Empty) {
	transactionIDs = make(map[ledgerstate.TransactionID]types.Empty)
	for _, input := range transaction.Essence().Inputs() {
		transactionIDs[input.(*ledgerstate.UTXOInput).ReferencedOutputID().TransactionID()] = types.Void
	}

	return
}

// Attachments retrieves the attachments of a transaction.
func (m *MessageBooker) Attachments(transactionID ledgerstate.TransactionID) (attachments MessageIDs) {
	return m.messageStore.AttachmentMessageIDs(transactionID)
}

func (m *MessageBooker) determineTargetBranch(branchIDsOfStrongParents ledgerstate.BranchIDs) (targetBranch ledgerstate.BranchID, err error) {
	if branchIDsOfStrongParents.Contains(ledgerstate.InvalidBranchID) {
		targetBranch = ledgerstate.InvalidBranchID
		return
	}

	branchIDsContainRejectedBranch, targetBranch := m.branchDAG.BranchIDsContainRejectedBranch(branchIDsOfStrongParents)
	if branchIDsContainRejectedBranch {
		return
	}

	cachedAggregatedBranch, _, err := m.branchDAG.AggregateBranches(branchIDsOfStrongParents)
	if err != nil {
		if xerrors.Is(err, ledgerstate.ErrInvalidStateTransition) {
			targetBranch = ledgerstate.InvalidBranchID
			return
		}

		err = xerrors.Errorf("failed to aggregate Branches of parent Messages: %w", err)
		return
	}
	cachedAggregatedBranch.Release()

	targetBranch = cachedAggregatedBranch.ID()
	return
}

func (m *MessageBooker) inheritStructureDetails(message *Message, branchID ledgerstate.BranchID, parentsStructureDetails []*markers.StructureDetails) (structureDetails *markers.StructureDetails) {
	structureDetails, _ = m.markersManager.InheritStructureDetails(parentsStructureDetails, m.newMarkerIndexStrategy, markers.NewSequenceAlias(branchID.Bytes()))

	if structureDetails.IsPastMarker {
		m.WalkPastCone(message.StrongParents(), m.propagatePastMarkerToFutureMarkers(structureDetails.PastMarkers.FirstMarker()))
	}

	return
}

func (m *MessageBooker) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageID MessageID) (nextMessagesToVisit MessageIDs) {
	return func(messageID MessageID) (nextMessagesToVisit MessageIDs) {
		m.messageStore.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			_, inheritFurther := m.markersManager.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
			if inheritFurther {
				m.messageStore.Message(messageID).Consume(func(message *Message) {
					nextMessagesToVisit = message.StrongParents()
				})
			}
		})

		return
	}
}

func (m *MessageBooker) WalkPastCone(entryPoints MessageIDs, callback func(messageID MessageID) MessageIDs) {
	stack := list.New()
	for messageID := range entryPoints {
		stack.PushBack(messageID)
	}

	for stack.Len() >= 1 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		for nextMessageID := range callback(firstElement.Value.(MessageID)) {
			stack.PushBack(nextMessageID)
		}
	}
}

// branchIDsOfStrongParents is an internal utility function that returns the BranchIDs of the strong parents of the
// given Message.
func (m *MessageBooker) branchIDsOfStrongParents(message *Message) (branchIDs ledgerstate.BranchIDs) {
	branchIDs = make(ledgerstate.BranchIDs)
	message.ForEachStrongParent(func(parent MessageID) {
		if !m.messageStore.MessageMetadata(parent).Consume(func(messageMetadata *MessageMetadata) {
			branchIDs[messageMetadata.BranchID()] = types.Void
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parent))
		}
	})

	return
}

// structuredDetailsOfStrongParents is an internal utility function that returns a list of StructureDetails of all the
// strong parents.
func (m MessageBooker) structuredDetailsOfStrongParents(message *Message) (structureDetails []*markers.StructureDetails) {
	structureDetails = make([]*markers.StructureDetails, 0)
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if !m.messageStore.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			structureDetails = append(structureDetails, messageMetadata.StructureDetails())
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata of Message with %s", parentMessageID))
		}
	})

	return
}
