package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	tangle *Tangle
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events: &BookerEvents{
			MessageBooked: events.NewEvent(messageIDEventHandler),
		},
		tangle: tangle,
	}

	return
}

// Book tries to book the given Message (and potentially its contained Transaction) into the LedgerState and the Tangle.
// It fires a MessageBooked event if it succeeds.
func (m *Booker) Book(messageID MessageID) (err error) {
	m.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		m.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			combinedBranches := m.branchIDsOfStrongParents(message)
			if payload := message.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				transaction := payload.(*ledgerstate.Transaction)
				if !m.tangle.LedgerState.TransactionValid(transaction, messageID) {
					return
				}

				if !m.allTransactionsApprovedByMessage(transaction.ReferencedTransactionIDs(), messageID) {
					m.tangle.Events.MessageInvalid.Trigger(messageID)
					return
				}

				targetBranch, bookingErr := m.tangle.LedgerState.BookTransaction(transaction, messageID)
				if bookingErr != nil {
					err = xerrors.Errorf("failed to book Transaction of Message with %s: %w", messageID, err)
					return
				}
				combinedBranches = combinedBranches.Add(targetBranch)
			}

			inheritedBranch, inheritErr := m.tangle.LedgerState.InheritBranch(combinedBranches)
			if inheritErr != nil {
				err = xerrors.Errorf("failed to inherit Branch when booking Message with %s: %w", message.ID(), inheritErr)
				return
			}

			messageMetadata.SetBranchID(inheritedBranch)
			messageMetadata.SetStructureDetails(m.tangle.MarkersManager.InheritStructureDetails(message, inheritedBranch, m.tangle.MarkersManager.structureDetailsOfStrongParents(message)))
			messageMetadata.SetBooked(true)

			m.Events.MessageBooked.Trigger(message.ID())
		})
	})

	return
}

// allTransactionsApprovedByMessage checks if all Transactions were attached by at least one Message that was directly
// or indirectly approved by the given Message.
func (m *Booker) allTransactionsApprovedByMessage(transactionIDs ledgerstate.TransactionIDs, messageID MessageID) (approved bool) {
	for transactionID := range transactionIDs {
		if !m.transactionApprovedByMessage(transactionID, messageID) {
			return false
		}
	}

	return true
}

// transactionApprovedByMessage checks if the Transaction was attached by at least one Message that was directly or
// indirectly approved by the given Message.
func (m *Booker) transactionApprovedByMessage(transactionID ledgerstate.TransactionID, messageID MessageID) (approved bool) {
	for _, attachmentMessageID := range m.tangle.Storage.AttachmentMessageIDs(transactionID) {
		if m.tangle.Utils.MessageApprovedBy(attachmentMessageID, messageID) {
			return true
		}
	}

	return false
}

// branchIDsOfStrongParents returns the BranchIDs of the strong parents of the given Message.
func (m *Booker) branchIDsOfStrongParents(message *Message) (branchIDs ledgerstate.BranchIDs) {
	branchIDs = make(ledgerstate.BranchIDs)
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if !m.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			branchIDs[messageMetadata.BranchID()] = types.Void
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parentMessageID))
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
