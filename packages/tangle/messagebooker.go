package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a manager that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	Events *BookerEvents

	tangle    *Tangle
	branchDAG *ledgerstate.BranchDAG
	utxoDAG   *ledgerstate.UTXODAG
}

// NewMessageBooker is the constructor of a Booker.
func NewMessageBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events: &BookerEvents{
			MessageBooked: events.NewEvent(messageIDEventHandler),
		},
		tangle: tangle,
	}

	return
}

func (m *Booker) Book(cachedMessage *CachedMessage, cachedMessageMetadata *CachedMessageMetadata) (err error) {
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
func (m *Booker) bookMessageContainingData(message *Message, messageMetadata *MessageMetadata) (err error) {
	targetBranch, err := m.determineTargetBranch(m.branchIDsOfStrongParents(message))
	if err != nil {
		err = xerrors.Errorf("failed to determine target Branch when booking Message with %s: %w", message.ID(), err)
		return
	}

	messageMetadata.SetBranchID(targetBranch)
	messageMetadata.SetStructureDetails(m.tangle.MarkersManager.InheritStructureDetails(message, targetBranch, m.tangle.MarkersManager.structureDetailsOfStrongParents(message)))
	messageMetadata.SetBooked(true)

	return
}

// TODO: ADD LAZY BOOKING STUFF

// bookMessageContainingTransaction is an internal utility function that books a Message containing a Transaction.
func (m *Booker) bookMessageContainingTransaction(message *Message, messageMetadata *MessageMetadata, transaction *ledgerstate.Transaction) {
	message.Parents()

	for referencedTransactionID := range m.referencedTransactionIDs(transaction) {
		for _, referencedMessageID := range m.Attachments(referencedTransactionID) {
			m.tangle.Storage.MessageMetadata(referencedMessageID).Consume(func(messageMetadata *MessageMetadata) {

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

func (m *Booker) referencedTransactionIDs(transaction *ledgerstate.Transaction) (transactionIDs map[ledgerstate.TransactionID]types.Empty) {
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

func (m *Booker) determineTargetBranch(branchIDsOfStrongParents ledgerstate.BranchIDs) (targetBranch ledgerstate.BranchID, err error) {
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

// branchIDsOfStrongParents is an internal utility function that returns the BranchIDs of the strong parents of the
// given Message.
func (m *Booker) branchIDsOfStrongParents(message *Message) (branchIDs ledgerstate.BranchIDs) {
	branchIDs = make(ledgerstate.BranchIDs)
	message.ForEachStrongParent(func(parent MessageID) {
		if !m.tangle.Storage.MessageMetadata(parent).Consume(func(messageMetadata *MessageMetadata) {
			branchIDs[messageMetadata.BranchID()] = types.Void
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parent))
		}
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// MessageBooked is triggered when a Message was booked (it's Branch and it's Payload's Branch where determined).
	MessageBooked *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
