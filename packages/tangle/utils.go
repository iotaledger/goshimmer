package tangle

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
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
func (u *Utils) WalkMessageID(callback func(messageID MessageID, walker *walker.Walker[MessageID]), entryPoints MessageIDs, revisitElements ...bool) {
	if len(entryPoints) == 0 {
		return
	}

	messageIDWalker := walker.New[MessageID](revisitElements...)
	for messageID := range entryPoints {
		messageIDWalker.Push(messageID)
	}

	for messageIDWalker.HasNext() {
		callback(messageIDWalker.Next(), messageIDWalker)
	}
}

// WalkMessage is a generic Tangle walker that executes a custom callback for every visited Message, starting from
// the given entry points. It accepts an optional boolean parameter which can be set to true if a Message should be
// visited more than once following different paths. The callback receives a Walker object as the last parameter which
// can be used to control the behavior of the walk similar to how a "Context" is used in some parts of the stdlib.
func (u *Utils) WalkMessage(callback func(message *Message, walker *walker.Walker[MessageID]), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker[MessageID]) {
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
func (u *Utils) WalkMessageMetadata(callback func(messageMetadata *MessageMetadata, walker *walker.Walker[MessageID]), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker[MessageID]) {
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
func (u *Utils) WalkMessageAndMetadata(callback func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker[MessageID]), entryPoints MessageIDs, revisitElements ...bool) {
	u.WalkMessageID(func(messageID MessageID, walker *walker.Walker[MessageID]) {
		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				callback(message, messageMetadata, walker)
			})
		})
	}, entryPoints, revisitElements...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region structural checks ////////////////////////////////////////////////////////////////////////////////////////////

// AllTransactionsApprovedByMessages checks if all Transactions were attached by at least one Message that was directly
// or indirectly approved by the given Message.
func (u *Utils) AllTransactionsApprovedByMessages(transactionIDs ledgerstate.TransactionIDs, messageIDs MessageIDs) (approved bool) {
	transactionIDs = transactionIDs.Clone()

	for messageID := range messageIDs {
		for transactionID := range transactionIDs {
			if u.TransactionApprovedByMessage(transactionID, messageID) {
				delete(transactionIDs, transactionID)
			}
		}
	}

	return len(transactionIDs) == 0
}

// TransactionApprovedByMessage checks if the Transaction was attached by at least one Message that was directly or
// indirectly approved by the given Message.
func (u *Utils) TransactionApprovedByMessage(transactionID ledgerstate.TransactionID, messageID MessageID) (approved bool) {
	attachmentMessageIDs := u.tangle.Storage.AttachmentMessageIDs(transactionID)
	for attachmentMessageID := range attachmentMessageIDs {
		if attachmentMessageID == messageID {
			return true
		}

		var attachmentBooked bool
		u.tangle.Storage.MessageMetadata(attachmentMessageID).Consume(func(attachmentMetadata *MessageMetadata) {
			attachmentBooked = attachmentMetadata.IsBooked()
		})
		if !attachmentBooked {
			continue
		}

		bookedParents := NewMessageIDs()

		u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			var tmpParents MessageIDs
			approved, tmpParents = u.checkBookedParents(message, attachmentMessageID, func(message *Message) MessageIDs {
				return message.ParentsByType(WeakParentType)
			})
			bookedParents.AddAll(tmpParents)
			if approved {
				return
			}

			approved, tmpParents = u.checkBookedParents(message, attachmentMessageID, func(message *Message) MessageIDs {
				return message.ParentsByType(StrongParentType)
			})
			bookedParents.AddAll(tmpParents)
		})
		if approved {
			return
		}

		// Only now check all parents.
		for bookedParent := range bookedParents {
			if u.MessageApprovedBy(attachmentMessageID, bookedParent) {
				approved = true
				return
			}
		}
		if approved {
			return
		}
	}

	return
}

// checkBookedParents check if message parents are booked and add then to bookedParents. If we find attachmentMessageId in the parents we stop and return true.
func (u *Utils) checkBookedParents(message *Message, attachmentMessageID MessageID, getParents func(*Message) MessageIDs) (bool, MessageIDs) {
	bookedParents := NewMessageIDs()

	for parentID := range getParents(message) {
		var parentBooked bool
		u.tangle.Storage.MessageMetadata(parentID).Consume(func(parentMetadata *MessageMetadata) {
			parentBooked = parentMetadata.IsBooked()
		})
		if !parentBooked {
			continue
		}

		// First check all of the parents to avoid unnecessary checks and possible walking.
		if attachmentMessageID == parentID {
			return true, bookedParents
		}

		bookedParents.Add(parentID)
	}
	return false, bookedParents
}

// MessageApprovedBy checks if the Message given by approvedMessageID is directly or indirectly approved by the
// Message given by approvingMessageID.
func (u *Utils) MessageApprovedBy(approvedMessageID MessageID, approvingMessageID MessageID) (approved bool) {
	if u.messageStronglyApprovedBy(approvedMessageID, approvingMessageID) {
		return true
	}

	cachedWeakApprovers := u.tangle.Storage.Approvers(approvedMessageID, WeakApprover)
	defer cachedWeakApprovers.Release()
	cachedShallowLikeApprovers := u.tangle.Storage.Approvers(approvedMessageID, ShallowLikeApprover)
	defer cachedShallowLikeApprovers.Release()
	cachedShallowDislikeApprovers := u.tangle.Storage.Approvers(approvedMessageID, ShallowDislikeApprover)
	defer cachedShallowDislikeApprovers.Release()

	indirectApprovers := append(cachedWeakApprovers.Unwrap(), append(cachedShallowLikeApprovers.Unwrap(), cachedShallowDislikeApprovers.Unwrap()...)...)

	for _, indirectApprover := range indirectApprovers {
		if indirectApprover == nil {
			continue
		}

		var indirectApproverBooked bool
		u.tangle.Storage.MessageMetadata(indirectApprover.ApproverMessageID()).Consume(func(indirectApproverMetadata *MessageMetadata) {
			indirectApproverBooked = indirectApproverMetadata.IsBooked()
		})
		if !indirectApproverBooked {
			continue
		}

		if u.messageStronglyApprovedBy(indirectApprover.ApproverMessageID(), approvingMessageID) {
			return true
		}
	}

	return false
}

// ApprovingMessageIDs returns the MessageIDs that approve a given Message. It accepts an optional ApproverType to
// filter the Approvers.
func (u *Utils) ApprovingMessageIDs(messageID MessageID, optionalApproverType ...ApproverType) (approvingMessageIDs MessageIDs) {
	approvingMessageIDs = NewMessageIDs()
	u.tangle.Storage.Approvers(messageID, optionalApproverType...).Consume(func(approver *Approver) {
		approvingMessageIDs.Add(approver.ApproverMessageID())
	})

	return
}

// AllBranchesLiked returs true if all the passed branches are liked.
func (u *Utils) AllBranchesLiked(branchIDs ledgerstate.BranchIDs) bool {
	for branchID := range branchIDs {
		if !u.tangle.OTVConsensusManager.BranchLiked(branchID) {
			return false
		}
	}

	return true
}

// messageStronglyApprovedBy checks if the Message given by approvedMessageID is directly or indirectly approved by the
// Message given by approvingMessageID (ignoring weak parents as a potential last reference).
func (u *Utils) messageStronglyApprovedBy(approvedMessageID MessageID, approvingMessageID MessageID) (stronglyApproved bool) {
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
		u.WalkMessageID(func(messageID MessageID, walker *walker.Walker[MessageID]) {
			if messageID == approvingMessageID {
				stronglyApproved = true
				walker.StopWalk()
				return
			}

			u.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
				if structureDetails := messageMetadata.StructureDetails(); structureDetails != nil && !structureDetails.IsPastMarker {
					for approvingMessageID := range u.tangle.Utils.ApprovingMessageIDs(messageID, StrongApprover) {
						walker.Push(approvingMessageID)
					}
				}
			})
		}, u.ApprovingMessageIDs(approvedMessageID, StrongApprover))
	}

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

// FirstAttachment returns the MessageID and timestamp of the first (oldest) attachment of a given transaction.
func (u *Utils) FirstAttachment(transactionID ledgerstate.TransactionID) (oldestAttachmentTime time.Time, oldestAttachmentMessageID MessageID, err error) {
	var transaction *ledgerstate.Transaction
	oldestAttachmentTime = time.Unix(0, 0)
	oldestAttachmentMessageID = EmptyMessageID
	if !u.tangle.Storage.Attachments(transactionID).Consume(func(attachment *Attachment) {
		u.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
			if oldestAttachmentTime.Unix() == 0 || message.IssuingTime().Before(oldestAttachmentTime) {
				oldestAttachmentTime = message.IssuingTime()
				oldestAttachmentMessageID = message.ID()
			}
			if transaction == nil {
				transaction = message.Payload().(*ledgerstate.Transaction)
			}
		})
	}) {
		err = errors.Errorf("could not find any attachments of transaction: %s", transactionID.String())
	}
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
