package tangle

import (
	"container/list"
	"time"

	"github.com/iotaledger/hive.go/types"
)

const maxParentAge = 10 * time.Minute

// SolidifyMessage starts solidify the given message.
func (t *Tangle) SolidifyMessage(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {
	// check message solidity
	t.checkMessageSolidityAndPropagate(cachedMessage, cachedMsgMetadata)
}

// checks whether the given message is solid and marks it as missing if it isn't known.
func (t *Tangle) isMessageMarkedAsSolid(messageID MessageID) bool {
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

// checks whether the given message is solid by examining whether its parent1 and
// parent2 messages are solid.
func (t *Tangle) isMessageSolid(msg *Message, msgMetadata *MessageMetadata) bool {
	if msg == nil || msg.IsDeleted() {
		return false
	}

	if msgMetadata == nil || msgMetadata.IsDeleted() {
		return false
	}

	if msgMetadata.IsSolid() {
		return true
	}

	solid := true

	msg.ForEachParent(func(parent Parent) {
		// as missing messages are requested in isMessageMarkedAsSolid,
		// we want to prevent short-circuit evaluation, thus we need to use a tmp variable
		// to avoid side effects from comparing directly to the function call.
		tmp := t.isMessageMarkedAsSolid(parent.ID)
		solid = solid && tmp
	})

	return solid
}

// builds up a stack from the given message and tries to solidify into the present.
// missing messages which are needed for a message to become solid are marked as missing.
func (t *Tangle) checkMessageSolidityAndPropagate(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {

	popElementsFromStack := func(stack *list.List) (*CachedMessage, *CachedMessageMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedMsg := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedMsgMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)
		return currentCachedMsg.(*CachedMessage), currentCachedMsgMetadata.(*CachedMessageMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedMessage, cachedMsgMetadata})

	// processed messages that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedMessage, currentCachedMsgMetadata := popElementsFromStack(solidificationStack)

		currentMessage := currentCachedMessage.Unwrap()
		currentMsgMetadata := currentCachedMsgMetadata.Unwrap()
		if currentMessage == nil || currentMsgMetadata == nil {
			currentCachedMessage.Release()
			currentCachedMsgMetadata.Release()
			continue
		}

		// check if the parents are solid
		if t.isMessageSolid(currentMessage, currentMsgMetadata) {
			// check if parents are valid and parents age
			valid := t.isParentsValid(currentMessage) && t.checkParentsMaxDepth(currentMessage)
			if !valid {
				// set the message invalid, trigger MessageInvalid event only if the msg is first set to invalid
				if currentMsgMetadata.SetInvalid(true) {
					t.Events.MessageInvalid.Trigger(&CachedMessageEvent{
						Message:         currentCachedMessage,
						MessageMetadata: currentCachedMsgMetadata})
				}
				currentCachedMessage.Release()
				currentCachedMsgMetadata.Release()
				continue
			}

			// set the message solid
			if currentMsgMetadata.SetSolid(true) {
				t.Events.MessageSolid.Trigger(&CachedMessageEvent{
					Message:         currentCachedMessage,
					MessageMetadata: currentCachedMsgMetadata})

				// auto. push approvers of the newly solid message to propagate solidification
				t.Approvers(currentMessage.ID()).Consume(func(approver *Approver) {
					approverMessageID := approver.ApproverMessageID()
					solidificationStack.PushBack([2]interface{}{
						t.Message(approverMessageID),
						t.MessageMetadata(approverMessageID),
					})
				})
			}
		}

		currentCachedMessage.Release()
		currentCachedMsgMetadata.Release()
	}
}

// deletes a message and its future cone of messages/approvers.
// nolint
func (t *Tangle) deleteFutureCone(messageID MessageID) {
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
func (t *Tangle) checkParentsMaxDepth(msg *Message) bool {
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
func (t *Tangle) isAgeOfParentValid(childTime time.Time, parentID MessageID) bool {
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
func (t *Tangle) isParentsValid(msg *Message) bool {
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
func (t *Tangle) isMessageValid(messageID MessageID) bool {
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
func (t *Tangle) CheckParentsEligibility(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {
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
func (t *Tangle) isMessageEligible(messageID MessageID) bool {
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
