package tangle

import (
	"container/list"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/types"
)

// OldTangle represents the base layer of messages.
type OldTangle struct {
	*Storage

	Events *Events
}

// Old creates an old Tangle.
func Old(store kvstore.KVStore) (result *OldTangle) {
	//result = &OldTangle{
	//	Storage: NewStorage(store),
	//	Events:       newEvents(),
	//}

	return
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
		t.Events.MessageEligible.Trigger(msgMetadata.ID())
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
