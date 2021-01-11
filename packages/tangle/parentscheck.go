package tangle

import (
	"time"
)

const maxParentAge = 10 * time.Minute

// CheckParentsEligibility checks if the parents are eligible and have valid timestamp(below max depth),
// then set the eligible flag of the message.
func (t *Tangle) CheckParentsEligibility(cachedMessage *CachedMessage, cachedMsgMetadata *CachedMessageMetadata) {
	msg := cachedMessage.Unwrap()
	msgMetadata := cachedMsgMetadata.Unwrap()
	defer msg.Release()
	defer msgMetadata.Release()
	// abort if the msg or msgMetadata does not exist.
	if msg == nil || msgMetadata == nil {
		return
	}

	eligible := true
	// First check if the parents are eligible
	msg.ForEachParent(func(parent Parent) {
		eligible = eligible && t.isMessageEligible(parent.ID)
	})

	// Second, check the below max depth
	eligible = eligible && t.checkParentsBelowMaxDepth(msg)

	// Lastly, set the eligible flag to the message
	msgMetadata.SetEligible(eligible)
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

// checks whether the given message is solid and marks it as missing if it isn't known.
func (t *Tangle) checkParentsBelowMaxDepth(msg *Message) bool {
	if msg == nil {
		return false
	}

	valid := true
	msg.ForEachParent(func(parent Parent) {
		valid = valid && t.isAgeOfParentValid(msg.IssuingTime(), parent.ID)
	})

	return valid
}

// checks whether the timestamp of a given parent passes the below max depth check.
func (t *Tangle) isAgeOfParentValid(childTime time.Time, parentID MessageID) bool {
	// retrieve the CachedMessage of parent
	parentCached := t.Message(parentID)
	defer parentCached.Release()

	// return false if the parent message does not exist
	parentMsg := parentCached.Unwrap()
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
