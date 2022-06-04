package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

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

// ApprovingMessageIDs returns the MessageIDs that approve a given Message. It accepts an optional ApproverType to
// filter the Approvers.
func (u *Utils) ApprovingMessageIDs(messageID MessageID, optionalApproverType ...ApproverType) (approvingMessageIDs MessageIDs) {
	approvingMessageIDs = NewMessageIDs()
	u.tangle.Storage.Approvers(messageID, optionalApproverType...).Consume(func(approver *Approver) {
		approvingMessageIDs.Add(approver.ApproverMessageID())
	})

	return
}

// AllBranchesLiked returns true if all the passed branches are liked.
func (u *Utils) AllBranchesLiked(branchIDs *set.AdvancedSet[utxo.TransactionID]) bool {
	for it := branchIDs.Iterator(); it.HasNext(); {
		if !u.tangle.OTVConsensusManager.BranchLiked(it.Next()) {
			return false
		}
	}

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// ComputeIfTransaction computes the given callback if the given messageID contains a transaction.
func (u *Utils) ComputeIfTransaction(messageID MessageID, compute func(utxo.TransactionID)) (computed bool) {
	u.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		if tx, ok := message.Payload().(utxo.Transaction); ok {
			transactionID := tx.ID()
			compute(transactionID)
			computed = true
		}
	})
	return
}

// FirstAttachment returns the MessageID and timestamp of the first (oldest) attachment of a given transaction.
func (u *Utils) FirstAttachment(transactionID utxo.TransactionID) (oldestAttachmentTime time.Time, oldestAttachmentMessageID MessageID, err error) {
	oldestAttachmentTime = time.Unix(0, 0)
	oldestAttachmentMessageID = EmptyMessageID
	if !u.tangle.Storage.Attachments(transactionID).Consume(func(attachment *Attachment) {
		u.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
			if oldestAttachmentTime.Unix() == 0 || message.IssuingTime().Before(oldestAttachmentTime) {
				oldestAttachmentTime = message.IssuingTime()
				oldestAttachmentMessageID = message.ID()
			}
		})
	}) {
		err = errors.Errorf("could not find any attachments of transaction: %s", transactionID.String())
	}
	return
}

// ConfirmedConsumer returns the confirmed transactionID consuming the given outputID.
func (u *Utils) ConfirmedConsumer(outputID utxo.OutputID) (consumerID utxo.TransactionID) {
	// default to no consumer, i.e. Genesis
	consumerID = utxo.EmptyTransactionID
	u.tangle.Ledger.Storage.CachedConsumers(outputID).Consume(func(consumer *ledger.Consumer) {
		if consumerID != utxo.EmptyTransactionID {
			return
		}
		if u.tangle.ConfirmationOracle.IsTransactionConfirmed(consumer.TransactionID()) {
			consumerID = consumer.TransactionID()
		}
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
