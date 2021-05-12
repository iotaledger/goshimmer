package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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

	tangle *Tangle
}

// NewSolidifier is the constructor of the Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: &SolidifierEvents{
			MessageSolid:   events.NewEvent(MessageIDCaller),
			MessageMissing: events.NewEvent(MessageIDCaller),
		},

		tangle: tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Solidifier) Setup() {
	s.tangle.Storage.Events.MessageStored.Attach(events.NewClosure(s.Solidify))
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.Utils.WalkMessageAndMetadata(s.checkMessageSolidity, MessageIDs{messageID}, true)
}

// checkMessageSolidity checks if the given Message is solid and eventually queues its Approvers to also be checked.
func (s *Solidifier) checkMessageSolidity(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker) {
	if !s.isMessageSolid(message, messageMetadata) {
		return
	}

	if !s.areParentMessagesValid(message) {
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

	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID, MessageSourceStrongParent)); stored {
			cachedMissingMessage.Consume(func(missingMessage *MissingMessage) {
				s.Events.MessageMissing.Trigger(messageID)
			})
		}

		// do not initialize the metadata here, we execute this in the optional ComputeIfAbsent callback to be secure
		// from race conditions
		return nil
	}).Consume(func(messageMetadata *MessageMetadata) {
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
		valid = valid && !messageMetadata.IsInvalid()
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// MessageWeaklySolid is triggered when a message becomes weakly solid, i.e. its weak references are solid and its
	// payload is solid.
	MessageWeaklySolid *events.Event

	// MessageSolid is triggered when a message becomes solid, i.e. all of its dependencies are known and solid.
	MessageSolid *events.Event

	// MessageMissing is triggered when a message references an unknown parent Message.
	MessageMissing *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region S0lidifier ////////////////////////////////////////////////////////////////////////////////////////////////

type MessageSource uint8

const (
	MessageSourceUndefined MessageSource = iota

	MessageSourceWeakParent

	MessageSourceStrongParent

	MessageSourceGossip
)

type S0lidifier struct {
	Events *SolidifierEvents
	tangle *Tangle
}

func (s *S0lidifier) OnTransactionSolid(transactionID ledgerstate.TransactionID) {
	s.propagateSolidity(s.tangle.Storage.AttachmentMessageIDs(transactionID)...)
}

func (s *S0lidifier) propagateSolidity(messageIDs ...MessageID) {
	s.tangle.Utils.WalkMessageAndMetadata(func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker) {
		if !s.isMessageWeaklySolid(message, messageMetadata, false) {
			return
		}

		var eventToTrigger *events.Event
		var approvingMessageIDs MessageIDs
		if s.isMessageSolid(message, messageMetadata, false) {
			if !messageMetadata.SetSolid(true) {
				return
			}

			eventToTrigger = s.Events.MessageSolid
			approvingMessageIDs = s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID())
		} else {
			if !messageMetadata.SetWeaklySolid(true) {
				return
			}

			eventToTrigger = s.Events.MessageWeaklySolid
			approvingMessageIDs = s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), WeakApprover)
		}

		eventToTrigger.Trigger(messageMetadata.ID())

		for _, messageID := range approvingMessageIDs {
			walker.Push(messageID)
		}
	}, messageIDs, true)
}

func (s *S0lidifier) Solidify(messageID MessageID, source MessageSource) {
	switch source {
	case MessageSourceGossip:
		s.StronglySolidify(messageID)
	case MessageSourceStrongParent:
		s.StronglySolidify(messageID)
	case MessageSourceWeakParent:
	}
}

func (s *S0lidifier) StronglySolidify(messageID MessageID) {
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if !messageMetadata.SetMessageSource(MessageSourceStrongParent) {
			return
		}

		s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			if s.isMessageSolid(message, messageMetadata, true) {
				s.triggerSolidUpdate(messageMetadata)
				return
			}

			if s.isMessageWeaklySolid(message, messageMetadata, false) {
				s.triggerWeaklySolidUpdate(messageMetadata)
			}
		})
	})
}

func (s *S0lidifier) WeaklySolidify(messageID MessageID) {
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			if !s.isMessageWeaklySolid(message, messageMetadata, true) {
				return
			}

			switch s.isMessageSolid(message, messageMetadata, false) {
			case false:
				s.triggerWeaklySolidUpdate(messageMetadata)
			case true:
				s.triggerSolidUpdate(messageMetadata)
			}
		})
	})
}

func (s *S0lidifier) isPayloadSolid(message *Message) (solid bool) {
	transaction, typeCastOkay := message.Payload().(*ledgerstate.Transaction)
	if !typeCastOkay {
		return true
	}

	isNew := false
	s.tangle.Storage.Attachment(transaction.ID(), message.ID(), func(transactionID ledgerstate.TransactionID, messageID MessageID) *Attachment {
		isNew = true

		attachment := NewAttachment(transactionID, messageID)
		attachment.SetModified()
		attachment.Persist()

		solid = s.tangle.LedgerState.UTXODAG.ProcessTransaction(transaction)

		return attachment
	})
	if isNew {
		return solid
	}

	s.tangle.LedgerState.UTXODAG.TransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		solid = transactionMetadata.Solid()
	})

	return solid
}

func (s *S0lidifier) isMessageWeaklySolid(message *Message, messageMetadata *MessageMetadata, requestMissingMessages bool) (weaklySolid bool) {
	if message == nil || message.IsDeleted() || messageMetadata == nil || messageMetadata.IsDeleted() {
		return false
	}

	if messageMetadata.IsWeaklySolid() || messageMetadata.IsSolid() {
		return true
	}

	if message.Payload().Type() != ledgerstate.TransactionType {
		return true
	}

	weaklySolid = true
	message.ForEachWeakParent(func(parentMessageID MessageID) {
		if weaklySolid || requestMissingMessages {
			weaklySolid = s.isMessageMarkedAsWeaklySolid(parentMessageID, requestMissingMessages) && weaklySolid
		}
	})
	if !weaklySolid {
		return false
	}

	return s.isPayloadSolid(message)
}

func (s *S0lidifier) isMessageSolid(message *Message, messageMetadata *MessageMetadata, requestMissingMessages bool) (solid bool) {
	if message == nil || message.IsDeleted() || messageMetadata == nil || messageMetadata.IsDeleted() {
		return false
	}

	if messageMetadata.IsSolid() {
		return true
	}

	solid = true
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if solid || requestMissingMessages {
			// as missing messages are requested in isMessageMarkedAsSolid, we need to be aware of short-circuit evaluation
			// rules, thus we need to evaluate isMessageMarkedAsSolid !!first!!
			solid = s.isMessageMarkedAsSolid(parentMessageID, requestMissingMessages) && solid
		}
	})
	message.ForEachWeakParent(func(parentMessageID MessageID) {
		if solid || requestMissingMessages {
			// as missing messages are requested in isMessageMarkedAsSolid, we need to be aware of short-circuit evaluation
			// rules, thus we need to evaluate isMessageMarkedAsSolid !!first!!
			solid = s.isMessageMarkedAsWeaklySolid(parentMessageID, requestMissingMessages) && solid
		}
	})
	if !solid {
		return false
	}

	return s.isPayloadSolid(message)
}

func (s *S0lidifier) isMessageMarkedAsSolid(messageID MessageID, requestMissingMessages bool) (solid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if !requestMissingMessages {
			return nil
		}

		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID, MessageSourceStrongParent)); stored {
			cachedMissingMessage.Consume(func(missingMessage *MissingMessage) {
				s.Events.MessageMissing.Trigger(messageID)
			})
		}

		// do not initialize the metadata here, we execute this in the optional ComputeIfAbsent callback to be secure
		// from race conditions
		return nil
	}).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid()
	})

	return
}

func (s *S0lidifier) isMessageMarkedAsWeaklySolid(messageID MessageID, requestMissingMessages bool) (solid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if !requestMissingMessages {
			return nil
		}

		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID, MessageSourceWeakParent)); stored {
			cachedMissingMessage.Consume(func(missingMessage *MissingMessage) {
				s.Events.MessageMissing.Trigger(messageID)
			})
		}

		// do not initialize the metadata here, we execute this in the optional ComputeIfAbsent callback to be secure
		// from race conditions
		return nil
	}).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid() || messageMetadata.IsWeaklySolid()
	})

	return
}

func (s *S0lidifier) triggerWeaklySolidUpdate(messageMetadata *MessageMetadata) {
	if !messageMetadata.SetWeaklySolid(true) {
		return
	}

	s.Events.MessageWeaklySolid.Trigger(messageMetadata.ID())

	s.propagateSolidity(s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), WeakApprover)...)
}

func (s *S0lidifier) triggerSolidUpdate(messageMetadata *MessageMetadata) {
	if !messageMetadata.SetSolid(true) {
		return
	}

	s.Events.MessageSolid.Trigger(messageMetadata.ID())

	s.propagateSolidity(s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID())...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
