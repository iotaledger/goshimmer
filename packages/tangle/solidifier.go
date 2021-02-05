package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/events"
)

const minParentsTimeDifference = 0 * time.Second

// maxParentsTimeDifference defines the cut-off condition for the maximum age of parent messages.
const maxParentsTimeDifference = 30 * time.Minute

// region Solidifier ///////////////////////////////////////////////////////////////////////////////////////////////////

// Solidifier is the Tangle's component that solidifies messages.
type Solidifier struct {
	// Events contains the Solidifier related events.
	Events *SolidifierEvents

	tangle *Tangle
}

// NewSolidifier is the constructor of Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: &SolidifierEvents{
			MessageSolid: events.NewEvent(messageIDEventHandler),
		},
		tangle: tangle,
	}

	return
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.WalkMessages(s.checkMessageSolidity, MessageIDs{messageID}, true)
}

// checkMessageSolidity checks if the given Message is solid and eventually queues its Approvers to also be checked.
func (s *Solidifier) checkMessageSolidity(message *Message, messageMetadata *MessageMetadata) (nextMessagesToCheck MessageIDs) {
	if !s.isMessageSolid(message, messageMetadata) {
		return
	}

	if !s.isParentsValid(message) || !s.checkParentsAge(message) {
		if !messageMetadata.SetInvalid(true) {
			return
		}

		s.tangle.Events.MessageInvalid.Trigger(message.ID())
		return
	}

	if !messageMetadata.SetSolid(true) {
		return
	}

	s.Events.MessageSolid.Trigger(message.ID())

	s.tangle.Storage.Approvers(message.ID()).Consume(func(approver *Approver) {
		nextMessagesToCheck = append(nextMessagesToCheck, approver.ApproverMessageID())
	})

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

// isMessageMarkedAsSolid checks whether the given message is solid and marks it as missing if it isn't known.
func (s *Solidifier) isMessageMarkedAsSolid(messageID MessageID) (solid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.StoreIfMissingMessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid()
	})

	return
}

// checkParentsAge checks whether the timestamp of each parent of the given message is valid.
func (s *Solidifier) checkParentsAge(message *Message) (valid bool) {
	if message == nil {
		return
	}

	valid = true
	message.ForEachParent(func(parent Parent) {
		valid = valid && s.isAgeOfParentValid(message.IssuingTime(), parent.ID)
	})

	return
}

// isAgeOfParentValid checks whether the timestamp of a given parent passes the max-age check.
func (s *Solidifier) isAgeOfParentValid(childMessageIssuingTime time.Time, parentID MessageID) (valid bool) {
	// TODO: Improve this, otherwise any msg that approves genesis is always valid.
	if parentID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.Message(parentID).Consume(func(parentMessage *Message) {
		timeDifference := childMessageIssuingTime.Sub(parentMessage.IssuingTime())

		valid = timeDifference >= minParentsTimeDifference && timeDifference <= maxParentsTimeDifference
	})

	return
}

// isParentsValid checks whether parents of the given message are valid.
func (s *Solidifier) isParentsValid(message *Message) (valid bool) {
	if message == nil || message.IsDeleted() {
		return false
	}

	valid = true
	message.ForEachParent(func(parent Parent) {
		valid = valid && s.isMessageValid(parent.ID)
	})

	return
}

// isMessageValid checks whether the given message is valid.
func (s *Solidifier) isMessageValid(messageID MessageID) (valid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		valid = !messageMetadata.IsInvalid()
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// MessageSolid is triggered when a message becomes solid, i.e. its past cone is known and solid.
	MessageSolid *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
