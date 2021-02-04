package tangle

import (
	"container/list"
	"time"

	"github.com/iotaledger/hive.go/types"
)

const maxParentAge = 10 * time.Minute

type Solidifier struct {
	tangle *Tangle
}

func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		tangle: tangle,
	}

	return
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.WalkMessages(s.checkMessageSolidity, messageID)
}

// checkMessageSolidity checks if the given Message is solid and eventually queues its Approvers to also be checked.
func (s *Solidifier) checkMessageSolidity(message *Message, messageMetadata *MessageMetadata) (nextMessagesToCheck MessageIDs) {
	if s.isMessageSolid(message, messageMetadata) {
		if !s.isParentsValid(message) || !s.checkParentsMaxDepth(message) {
			if messageMetadata.SetInvalid(true) {
				s.tangle.Events.MessageInvalid.Trigger(message.ID())
			}
			return
		}

		if messageMetadata.SetSolid(true) {
			s.tangle.Events.MessageSolid.Trigger(message.ID())

			s.tangle.Storage.Approvers(message.ID()).Consume(func(approver *Approver) {
				nextMessagesToCheck = append(nextMessagesToCheck, approver.ApproverMessageID())
			})
		}
	}

	return
}

// isMessageSolid checks if the given Message is solid.
func (s *Solidifier) isMessageSolid(message *Message, messageMetadata *MessageMetadata) (solid bool) {
	if message == nil || message.IsDeleted() || messageMetadata == nil || messageMetadata.IsDeleted() {
		return false
	}

	if messageMetadata.IsSolid() {
		return true
	}

	solid = true
	message.ForEachParent(func(parent Parent) {
		// as missing messages are requested in isMessageMarkedAsSolid, we need to be aware of short-circuit evaluation
		// rules, thus we need to evaluate isMessageMarkedAsSolid !!first!!
		solid = s.isMessageMarkedAsSolid(parent.ID) && solid
	})

	return
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (s *Solidifier) isMessageMarkedAsSolid(messageID MessageID) bool {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata and trigger the MessageMissing event if it doesn't exist
	msgMetadataCached := t.StoreIfMissingMessageMetadata(messageID)
	defer msgMetadataCached.Release()

	// return false if the metadata does not exist
	msgMetadata := msgMetadataCached.Unwrap()
	if msgMetadata == nil {
		return false
	}

	// return the solid flag of the metadata object
	return msgMetadata.IsSolid()
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (t *OldTangle) checkMessageSolidityAndPropagate(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

}

// deletes a message and its future cone of messages/approvers.
// nolint
func (t *OldTangle) deleteFutureCone(messageID MessageID) {
	cleanupStack := list.New()
	cleanupStack.PushBack(messageID)

	processedMessages := make(map[MessageID]types.Empty)
	processedMessages[messageID] = types.Void

	for cleanupStack.Len() >= 1 {
		currentStackEntry := cleanupStack.Front()
		currentMessageID := currentStackEntry.Value.(MessageID)
		cleanupStack.Remove(currentStackEntry)

		t.DeleteMessage(currentMessageID)

		t.Approvers(currentMessageID).Consume(func(approver *Approver) {
			approverID := approver.ApproverMessageID()
			if _, messageProcessed := processedMessages[approverID]; !messageProcessed {
				cleanupStack.PushBack(approverID)
				processedMessages[approverID] = types.Void
			}
		})
	}
}

// checks whether the timestamp of parents of the given message is valid.
func (t *OldTangle) checkParentsMaxDepth(msg *Message) bool {
	if msg == nil {
		return false
	}

	valid := true
	msg.ForEachParent(func(parent Parent) {
		valid = valid && t.isAgeOfParentValid(msg.IssuingTime(), parent.ID)
	})

	return valid
}

// checks whether the timestamp of a given parent passes the max age check.
func (t *OldTangle) isAgeOfParentValid(childTime time.Time, parentID MessageID) bool {
	// TODO: Improve this, otherwise any msg that approves genesis is always valid.
	if parentID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessage of parent
	parentCachedMsg := t.Message(parentID)
	defer parentCachedMsg.Release()

	// return false if the parent message does not exist
	parentMsg := parentCachedMsg.Unwrap()
	if parentMsg == nil {
		return false
	}

	// check the parent is not too young
	if !parentMsg.IssuingTime().Before(childTime) {
		return false
	}

	// check the parent is not too old
	if childTime.Sub(parentMsg.IssuingTime()) > maxParentAge {
		return false
	}

	return true
}

// checks whether parents of the given message are valid.
func (t *OldTangle) isParentsValid(msg *Message) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	valid := true
	msg.ForEachParent(func(parent Parent) {
		valid = valid && t.isMessageValid(parent.ID)
	})

	return valid
}

// checks whether the given message is valid.
func (t *OldTangle) isMessageValid(messageID MessageID) bool {
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata
	cachedMsgMetadata := t.MessageMetadata(messageID)
	defer cachedMsgMetadata.Release()

	// return false if the message metadata does not exist
	msgMetadata := cachedMsgMetadata.Unwrap()
	if msgMetadata == nil {
		return false
	}

	return !msgMetadata.IsInvalid()
}

// CheckParentsEligibility checks if the parents are eligible, then set the eligible flag of the message.
// TODO: Eligibility related functions will be moved elsewhere.
func (t *OldTangle) CheckParentsEligibility(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMsgMetadata.Release()

	msg := cachedMessage.Unwrap()
	msgMetadata := cachedMsgMetadata.Unwrap()
	// abort if the msg or msgMetadata does not exist.
	if msg == nil || msgMetadata == nil {
		return
	}

	eligible := true
	// First check if the parents are eligible
	msg.ForEachParent(func(parent Parent) {
		eligible = eligible && t.isMessageEligible(parent.ID)
	})

	// Lastly, set the eligible flag and trigger event
	if eligible && msgMetadata.SetEligible(eligible) {
		t.Events.MessageEligible.Trigger(&CachedMessageEvent{
			Message:         cachedMessage,
			MessageMetadata: cachedMsgMetadata,
		})
	}
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (t *OldTangle) isMessageEligible(messageID MessageID) bool {
	// return true if the message is the Genesis
	if messageID == EmptyMessageID {
		return true
	}

	// retrieve the CachedMessageMetadata
	msgMetadataCached := t.MessageMetadata(messageID)
	defer msgMetadataCached.Release()

	// return false if the metadata does not exist
	msgMetadata := msgMetadataCached.Unwrap()
	if msgMetadata == nil {
		return false
	}

	// return the solid flag of the metadata object
	return msgMetadata.IsEligible()
}
