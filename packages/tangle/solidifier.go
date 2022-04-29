package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/debug"
	"github.com/iotaledger/hive.go/generics/event"
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

	triggerMutex syncutils.MultiMutex
	tangle       *Tangle
}

// NewSolidifier is the constructor of the Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: newSolidifierEvents(),
		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Solidifier) Setup() {
	s.tangle.Storage.Events.MessageStored.Hook(event.NewClosure(func(event *MessageStoredEvent) {
		s.solidify(event.Message)
	}))

	s.Events.MessageSolid.Attach(event.NewClosure(func(event *MessageSolidEvent) {
		s.processApprovers(event.Message.ID())
	}))
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.Storage.Message(messageID).Consume(s.solidify)
}

// Solidify solidifies the given Message.
func (s *Solidifier) solidify(message *Message) {
	s.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		s.checkMessageSolidity(message, messageMetadata)
	})
}

func (s *Solidifier) processApprovers(messageID MessageID) {
	s.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		event.Loop.Submit(func() {
			s.Solidify(approver.ApproverMessageID())
		})
	})
}

// RetrieveMissingMessage checks if the message is missing and triggers the corresponding events to request it. It returns true if the message has been missing.
func (s *Solidifier) RetrieveMissingMessage(messageID MessageID) (messageWasMissing bool) {
	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID)); stored {
			cachedMissingMessage.Release()

			messageWasMissing = true
			s.Events.MessageMissing.Trigger(&MessageMissingEvent{messageID})
		}

		return nil
	}).Release()

	return messageWasMissing
}

// checkMessageSolidity checks if the given Message is solid and eventually queues its Approvers to also be checked.
func (s *Solidifier) checkMessageSolidity(message *Message, messageMetadata *MessageMetadata) {
	s.tangle.dagMutex.RLock(message.Parents()...)
	defer s.tangle.dagMutex.RUnlock(message.Parents()...)
	s.tangle.dagMutex.Lock(message.ID())
	defer s.tangle.dagMutex.Unlock(message.ID())

	if !s.isMessageSolid(message, messageMetadata) {
		fmt.Println(debug.GoroutineID(), "HIE0R2", message.ID())

		return
	}

	fmt.Println(debug.GoroutineID(), "HIE0R3", message.ID())

	if !s.areParentMessagesValid(message) {
		if !messageMetadata.SetObjectivelyInvalid(true) {
			fmt.Println(debug.GoroutineID(), "HIE0R4", message.ID())

			return
		}

		fmt.Println(debug.GoroutineID(), "HIE0R5", message.ID())

		s.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: message.ID(), Error: ErrParentsInvalid})

		fmt.Println(debug.GoroutineID(), "HIE0R6", message.ID())

		return
	}

	fmt.Println(debug.GoroutineID(), "HIE0R7", message.ID())

	if !messageMetadata.SetSolid(true) {
		fmt.Println(debug.GoroutineID(), "HIE0R8", message.ID())

		return
	}

	fmt.Println(debug.GoroutineID(), "Solidified", message.ID())

	s.Events.MessageSolid.Trigger(&MessageSolidEvent{message})

	fmt.Println(debug.GoroutineID(), "Solidified Triggered", message.ID())
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
