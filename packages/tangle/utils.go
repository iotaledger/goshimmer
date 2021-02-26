package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/types"
)

// region Utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Utils is a Tangle component that bundles methods that can be used to interact with the Tangle, that do not belong
// into public API.
type Utils struct {
	tangle *Tangle
}

// NewUtils is the constructor of the Utils component.
func NewUtils(tangle *Tangle) (utils *Utils) {
	return &Utils{
		tangle: tangle,
	}
}

// region walkers //////////////////////////////////////////////////////////////////////////////////////////////////////

// WalkMessageID is a generic Tangle walker that executes a custom callback for every visited MessageID, starting from
// the given entry points. It accepts an optional boolean parameter which can be set to true if a Message should be
// visited more than once following different paths. The callback receives a Walker object as the last parameter which
// can be used to control the behavior of the walk similar to how a "Context" is used in some parts of the stdlib.
func (u *Utils) WalkMessageID(callback func(messageID MessageID, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	if len(entryPoints) == 0 {
		return
	}

	messageIDWalker := walker.New(revisitElements...)
	for _, messageID := range entryPoints {
		messageIDWalker.Push(messageID)
	}

	for messageIDWalker.HasNext() {
		callback(messageIDWalker.Next().(MessageID), messageIDWalker)
	}
}

// WalkMessage is a generic Tangle walker that executes a custom callback for every visited Message, starting from
// the given entry points. It accepts an optional boolean parameter which can be set to true if a Message should be
// visited more than once following different paths. The callback receives a Walker object as the last parameter which
// can be used to control the behavior of the walk similar to how a "Context" is used in some parts of the stdlib.
func (u *Utils) WalkMessage(callback func(message *Message, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			callback(message, walker)
		})
	}, entryPoints, revisitElements...)
}

// WalkMessageMetadata is a generic Tangle walker that executes a custom callback for every visited MessageMetadata,
// starting from the given entry points. It accepts an optional boolean parameter which can be set to true if a Message
// should be visited more than once following different paths. The callback receives a Walker object as the last
// parameter which can be used to control the behavior of the walk similar to how a "Context" is used in some parts of
// the stdlib.
func (u *Utils) WalkMessageMetadata(callback func(messageMetadata *MessageMetadata, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			callback(messageMetadata, walker)
		})
	}, entryPoints, revisitElements...)
}

// WalkMessageAndMetadata is a generic Tangle walker that executes a custom callback for every visited Message and
// MessageMetadata, starting from the given entry points. It accepts an optional boolean parameter which can be set to
// true if a Message should be visited more than once following different paths. The callback receives a Walker object
// as the last parameter which can be used to control the behavior of the walk similar to how a "Context" is used in
// some parts of the stdlib.
func (u *Utils) WalkMessageAndMetadata(callback func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				callback(message, messageMetadata, walker)
			})
		})
	}, entryPoints, revisitElements...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region structural checks ////////////////////////////////////////////////////////////////////////////////////////////

// AllTransactionsApprovedByMessage checks if all Transactions were attached by at least one Message that was directly
// or indirectly approved by the given Message.
func (u *Utils) AllTransactionsApprovedByMessage(transactionIDs ledgerstate.TransactionIDs, messageID MessageID) (approved bool) {
	for transactionID := range transactionIDs {
		if !u.TransactionApprovedByMessage(transactionID, messageID) {
			return false
		}
	}

	return true
}

// AllTransactionsContainedOrApprovedByMessages checks if all Transactions were attached by at least one Message that was directly
// or indirectly approved by the given Messages.
func (u *Utils) AllTransactionsContainedOrApprovedByMessages(transactionIDs ledgerstate.TransactionIDs, messageIDs MessageIDs) (approved bool) {
	// keep track of already approved transactions
	approvedTransactions := make(map[ledgerstate.TransactionID]bool)
	for transactionID := range transactionIDs {
		approvedTransactions[transactionID] = false
	}

	// check if transactions are contained in messages
	for _, messageID := range messageIDs {
		for transactionID := range transactionIDs {
			// no need to check if it's already contained in another message
			if approvedTransactions[transactionID] {
				continue
			}

			approvedTransactions[transactionID] = u.tangle.Storage.IsTransactionAttachedByMessage(transactionID, messageID)
		}
	}

	for _, messageID := range messageIDs {
		for transactionID := range transactionIDs {
			// no need to check if it's already approved by another message
			if approvedTransactions[transactionID] {
				continue
			}

			//fmt.Println("Checking with message", messageID)
			approvedTransactions[transactionID] = u.TransactionApprovedByMessage(transactionID, messageID)
		}
	}

	for transactionID := range approvedTransactions {
		if !approvedTransactions[transactionID] {
			return false
		}
	}
	return true
}

// TransactionApprovedByMessage checks if the Transaction was attached by at least one Message that was directly or
// indirectly approved by the given Message.
func (u *Utils) TransactionApprovedByMessage(transactionID ledgerstate.TransactionID, messageID MessageID) (approved bool) {
	for _, attachmentMessageID := range u.tangle.Storage.AttachmentMessageIDs(transactionID) {
		if u.tangle.Utils.MessageApprovedByStrongParents(attachmentMessageID, messageID) {
			return true
		}
	}

	return false
}

// MessageApprovedBy checks if the Message given by approvedMessageID is directly or indirectly approved by the
// Message given by approvingMessageID.
func (u *Utils) MessageApprovedBy(approvedMessageID MessageID, approvingMessageID MessageID) (approved bool) {
	if u.MessageStronglyApprovedBy(approvedMessageID, approvingMessageID) {
		return true
	}

	cachedWeakApprovers := u.tangle.Storage.Approvers(approvedMessageID, WeakApprover)
	defer cachedWeakApprovers.Release()

	for _, weakApprover := range cachedWeakApprovers.Unwrap() {
		if weakApprover == nil {
			continue
		}

		if u.MessageStronglyApprovedBy(weakApprover.ApproverMessageID(), approvingMessageID) {
			return true
		}
	}

	return false
}

// MessageApprovedByStrongParents checks if the Message given by approvedMessageID is directly or indirectly approved by its strong parents.
func (u *Utils) MessageApprovedByStrongParents(approvedMessageID MessageID, approvingMessageID MessageID) (approved bool) {
	u.tangle.Storage.Message(approvingMessageID).Consume(func(message *Message) {
		for _, parentID := range message.StrongParents() {
			if u.MessageApprovedBy(approvedMessageID, parentID) {
				approved = true
				return
			}
		}
	})
	return
}

// MessageStronglyApprovedBy checks if the Message given by approvedMessageID is directly or indirectly approved by the
// Message given by approvingMessageID (ignoring weak parents as a potential last reference).
func (u *Utils) MessageStronglyApprovedBy(approvedMessageID MessageID, approvingMessageID MessageID) (stronglyApproved bool) {
	if approvedMessageID == approvingMessageID || approvedMessageID == EmptyMessageID {
		return true
	}

	var approvedMessageStructureDetails *markers.StructureDetails
	u.tangle.Storage.MessageMetadata(approvedMessageID).Consume(func(messageMetadata *MessageMetadata) {
		approvedMessageStructureDetails = messageMetadata.StructureDetails()
	})
	if approvedMessageStructureDetails == nil {
		panic(fmt.Sprintf("tried to check approval of non-booked Message with %s", approvedMessageID))
	}

	var approvingMessageStructureDetails *markers.StructureDetails
	u.tangle.Storage.MessageMetadata(approvingMessageID).Consume(func(messageMetadata *MessageMetadata) {
		approvingMessageStructureDetails = messageMetadata.StructureDetails()
	})
	if approvingMessageStructureDetails == nil {
		panic(fmt.Sprintf("tried to check approval of non-booked Message with %s", approvingMessageID))
	}

	switch u.tangle.Booker.MarkersManager.IsInPastCone(approvedMessageStructureDetails, approvingMessageStructureDetails) {
	case types.True:
		stronglyApproved = true
	case types.False:
		stronglyApproved = false
	case types.Maybe:
		u.WalkMessageID(func(messageID MessageID, walker *walker.Walker) {
			if messageID == approvingMessageID {
				stronglyApproved = true
				walker.StopWalk()
				return
			}

			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				if structureDetails := messageMetadata.StructureDetails(); structureDetails != nil && !structureDetails.IsPastMarker {
					for _, approvingMessageID := range u.tangle.Utils.ApprovingMessageIDs(messageID, StrongApprover) {
						walker.Push(approvingMessageID)
					}
				}
			})
		}, u.ApprovingMessageIDs(approvedMessageID, StrongApprover))
	}

	return
}

// ApprovingMessageIDs returns the MessageIDs that approve a given Message. It accepts an optional ApproverType to
// filter the Approvers.
func (u *Utils) ApprovingMessageIDs(messageID MessageID, optionalApproverType ...ApproverType) (approvingMessageIDs MessageIDs) {
	approvingMessageIDs = make(MessageIDs, 0)
	u.tangle.Storage.Approvers(messageID, optionalApproverType...).Consume(func(approver *Approver) {
		approvingMessageIDs = append(approvingMessageIDs, approver.ApproverMessageID())
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// ComputeIfTransaction computes the given callback if the given messageID contains a transaction.
func (u *Utils) ComputeIfTransaction(messageID MessageID, compute func(ledgerstate.TransactionID)) (computed bool) {
	u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if payload := message.Payload(); payload.Type() == ledgerstate.TransactionType {
			transactionID := payload.(*ledgerstate.Transaction).ID()
			compute(transactionID)
			computed = true
		}
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
