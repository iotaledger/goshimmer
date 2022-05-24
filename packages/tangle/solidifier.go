package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/syncutils"
)

// maxParentsTimeDifference defines the smallest allowed time difference between a child Message and its parents.
const minParentsTimeDifference = 0 * time.Second

// maxParentsTimeDifference defines the biggest allowed time difference between a child Message and its parents.
const maxParentsTimeDifference = 30 * time.Minute

// region Solidifier ///////////////////////////////////////////////////////////////////////////////////////////////////

// Solidifier is the Tangle's component that solidifies messages.
type Solidifier struct {
	// Events contains the Solidifier related events.
	Events *SolidifierEvents

	triggerMutex *syncutils.MultiMutex
	tangle       *Tangle
}

// NewSolidifier is the constructor of the Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: &SolidifierEvents{
			MessageSolid:   events.NewEvent(MessageIDCaller),
			MessageMissing: events.NewEvent(MessageIDCaller),
		},
		triggerMutex: syncutils.NewMultiMutex(),
		tangle:       tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Solidifier) Setup() {
	s.tangle.Storage.Events.MessageStored.Attach(events.NewClosure(s.Solidify))
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.Utils.WalkMessageAndMetadata(s.checkMessageSolidity, NewMessageIDs(messageID), true)
}

// RetrieveMissingMessage checks if the message is missing and triggers the corresponding events to request it. It returns true if the message has been missing.
func (s *Solidifier) RetrieveMissingMessage(messageID MessageID) (messageWasMissing bool) {
	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID)); stored {
			cachedMissingMessage.Release()

			messageWasMissing = true
			s.Events.MessageMissing.Trigger(messageID)
		}

		return nil
	}).Release()

	return messageWasMissing
}

// checkMessageSolidity checks if the given Message is solid and eventually queues its Approvers to also be checked.
func (s *Solidifier) checkMessageSolidity(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker[MessageID]) {
	if !s.isMessageSolid(message, messageMetadata) {
		return
	}

	if !s.areParentMessagesValid(message) {
		if !messageMetadata.SetObjectivelyInvalid(true) {
			return
		}
		s.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: message.ID(), Error: ErrParentsInvalid})
		return
	}

	lockBuilder := syncutils.MultiMutexLockBuilder{}
	lockBuilder.AddLock(messageMetadata.ID())

	message.ForEachParent(func(parent Parent) {
		lockBuilder.AddLock(parent.ID)
	})
	lock := lockBuilder.Build()

	s.triggerMutex.Lock(lock...)
	defer s.triggerMutex.Unlock(lock...)

	if !messageMetadata.SetSolid(true) {
		return
	}
	s.Events.MessageSolid.Trigger(message.ID())

	s.tangle.Storage.Approvers(message.ID()).Consume(func(approver *Approver) {
		walker.Push(approver.ApproverMessageID())
	})
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

	if s.RetrieveMissingMessage(messageID) {
		return false
	}

	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid()
	})

	return
}

// areParentMessagesValid checks whether the parents of the given Message are valid.
func (s *Solidifier) areParentMessagesValid(message *Message) (valid bool) {
	valid = true
	message.ForEachParent(func(parent Parent) {
		valid = valid && s.isParentMessageValid(parent.ID, message)
	})

	return
}

// isParentMessageValid checks whether the given parent Message is valid.
func (s *Solidifier) isParentMessageValid(parentMessageID MessageID, childMessage *Message) (valid bool) {
	if parentMessageID == EmptyMessageID {
		if s.tangle.Options.GenesisNode != nil {
			return *s.tangle.Options.GenesisNode == childMessage.IssuerPublicKey()
		}

		s.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			timeDifference := childMessage.IssuingTime().Sub(messageMetadata.SolidificationTime())
			valid = timeDifference >= minParentsTimeDifference && timeDifference <= maxParentsTimeDifference
		})
		return
	}

	s.tangle.Storage.Message(parentMessageID).Consume(func(parentMessage *Message) {
		timeDifference := childMessage.IssuingTime().Sub(parentMessage.IssuingTime())

		valid = timeDifference >= minParentsTimeDifference && timeDifference <= maxParentsTimeDifference
	})

	s.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
		valid = valid && !messageMetadata.IsObjectivelyInvalid()
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// MessageSolid is triggered when a message becomes solid, i.e. its past cone is known and solid.
	MessageSolid *events.Event

	// MessageMissing is triggered when a message references an unknown parent Message.
	MessageMissing *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
