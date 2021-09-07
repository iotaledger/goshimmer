package tangle

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/eventsqueue"
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

	payloadsSolidTriggered      map[ledgerstate.TransactionID]types.Empty
	payloadsSolidTriggeredMutex sync.RWMutex
	triggerMutex                syncutils.MultiMutex
	tangle                      *Tangle
}

// NewSolidifier is the constructor of the Solidifier.
func NewSolidifier(tangle *Tangle) (solidifier *Solidifier) {
	solidifier = &Solidifier{
		Events: &SolidifierEvents{
			Error:              events.NewEvent(events.ErrorCaller),
			MessageWeaklySolid: events.NewEvent(MessageIDCaller),
			MessageSolid:       events.NewEvent(MessageIDCaller),
			MessageMissing:     events.NewEvent(MessageIDCaller),
		},

		payloadsSolidTriggered: make(map[ledgerstate.TransactionID]types.Empty),
		tangle:                 tangle,
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Solidifier) Setup() {
	s.tangle.Storage.Events.MessageStored.Attach(events.NewClosure(s.Solidify))
	s.tangle.LedgerState.UTXODAG.Events().TransactionSolid.Attach(events.NewClosure(s.TriggerTransactionSolid))
}

func (s *Solidifier) setPayloadSolidificationRunning(transactionID ledgerstate.TransactionID, running bool) {
	s.payloadsSolidTriggeredMutex.Lock()
	defer s.payloadsSolidTriggeredMutex.Unlock()

	if running {
		s.payloadsSolidTriggered[transactionID] = types.Void
	} else {
		delete(s.payloadsSolidTriggered, transactionID)
	}

}

func (s *Solidifier) payloadSolidificationRunning(transactionID ledgerstate.TransactionID) (hasPayloadSolidTriggered bool) {
	s.payloadsSolidTriggeredMutex.RLock()
	defer s.payloadsSolidTriggeredMutex.RUnlock()

	_, hasPayloadSolidTriggered = s.payloadsSolidTriggered[transactionID]

	return
}

// TriggerTransactionSolid triggers the solidity checks related to a transaction payload becoming solid.
func (s *Solidifier) TriggerTransactionSolid(transactionID ledgerstate.TransactionID) {
	if s.payloadSolidificationRunning(transactionID) {
		return
	}

	s.propagateSolidity(s.tangle.Storage.AttachmentMessageIDs(transactionID)...)
}

// Solidify solidifies the given Message.
func (s *Solidifier) Solidify(messageID MessageID) {
	s.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		solidificationType := messageMetadata.Source().SolidificationType()
		if solidificationType == UndefinedSolidificationType {
			return
		}

		s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			s.solidify(message, messageMetadata, solidificationType)
		})
	})
}

func (s *Solidifier) solidifyWeakly(message *Message, messageMetadata *MessageMetadata, requestMissingMessages bool) (approversToPropagate MessageIDs) {
	eventsQueue := eventsqueue.New()
	defer eventsQueue.Trigger()

	fmt.Println("LOCK", message.Locks())
	defer fmt.Println("UNLOCK", message.Locks())

	s.triggerMutex.LockEntity(message)
	defer s.triggerMutex.UnlockEntity(message)

	approversToPropagate = make(MessageIDs, 0)

	if !s.isMessageWeaklySolid(message, messageMetadata, requestMissingMessages, eventsQueue) {
		return
	}

	if s.isMessageSolid(message, messageMetadata, false, eventsQueue) {
		if s.triggerSolidUpdate(messageMetadata, eventsQueue) {
			for _, messageID := range s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID()) {
				approversToPropagate = append(approversToPropagate, messageID)
			}
		}

		return
	}

	if s.triggerWeaklySolidUpdate(messageMetadata, eventsQueue) {
		for _, messageID := range s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID()) {
			approversToPropagate = append(approversToPropagate, messageID)
		}
	}

	return
}

func (s *Solidifier) solidifyStrongly(message *Message, messageMetadata *MessageMetadata) (approversToPropagate MessageIDs) {
	eventsQueue := eventsqueue.New()
	defer eventsQueue.Trigger()

	fmt.Println("LOCKING", message.Locks())
	defer fmt.Println("UNLOCKED", message.Locks())

	s.triggerMutex.LockEntity(message)
	defer s.triggerMutex.UnlockEntity(message)

	fmt.Println("LOCKED", message.Locks())

	approversToPropagate = make(MessageIDs, 0)

	if s.isMessageSolid(message, messageMetadata, true, eventsQueue) {
		if s.triggerSolidUpdate(messageMetadata, eventsQueue) {
			for _, messageID := range s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID()) {
				approversToPropagate = append(approversToPropagate, messageID)
			}
		}

		return
	}

	if s.isMessageWeaklySolid(message, messageMetadata, false, eventsQueue) {
		if s.triggerWeaklySolidUpdate(messageMetadata, eventsQueue) {
			for _, messageID := range s.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID()) {
				approversToPropagate = append(approversToPropagate, messageID)
			}
		}
	}

	return
}

// solidify performs the solidity checks and triggers the corresponding updates.
func (s *Solidifier) solidify(message *Message, messageMetadata *MessageMetadata, solidificationType SolidificationType) {
	if !messageMetadata.SetSolidificationType(solidificationType) {
		return
	}

	switch solidificationType {
	case WeakSolidification:
		s.propagateSolidity(s.solidifyWeakly(message, messageMetadata, true)...)
	case StrongSolidification:
		s.propagateSolidity(s.solidifyStrongly(message, messageMetadata)...)
	}
}

// isMessageWeaklySolid checks if the given Message is weakly solid.
//
// Note: A message is weakly solid if it is either not containing a transaction payload or if its weak parents are at
//       least weakly solid (and pass the parents age check - otherwise the message is invalid).
func (s *Solidifier) isMessageWeaklySolid(message *Message, messageMetadata *MessageMetadata, requestMissingMessages bool, eventsQueue *eventsqueue.EventsQueue) (weaklySolid bool) {
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
	message.ForEachParentByType(WeakParentType, func(parentMessageID MessageID) {
		if weaklySolid || requestMissingMessages {
			weaklySolid = s.isMessageMarkedAsWeaklySolid(parentMessageID, requestMissingMessages, eventsQueue) && weaklySolid
		}
	})
	if !weaklySolid {
		return false
	}

	if !s.parentsAgeValid(message.IssuingTime(), message.ParentsByType(WeakParentType)) {
		if messageMetadata.SetInvalid(true) {
			eventsQueue.Queue(s.tangle.Events.MessageInvalid, message.ID())
		}

		return false
	}

	return s.solidifyPayload(message, messageMetadata, eventsQueue)
}

// isMessageSolid checks if the given Message is solid.
//
// Note: A message is solid if its strong parents are solid and its weak parents are at least weakly solid. If any of
//       the parents doesn't pass the parents age check then the Message is invalid.
func (s *Solidifier) isMessageSolid(message *Message, messageMetadata *MessageMetadata, requestMissingMessages bool, eventsQueue *eventsqueue.EventsQueue) (solid bool) {
	if message == nil || message.IsDeleted() || messageMetadata == nil || messageMetadata.IsDeleted() {
		return false
	}

	if messageMetadata.IsSolid() {
		return true
	}

	solid = true
	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) {
		if solid || requestMissingMessages {
			solid = s.isMessageMarkedAsSolid(parentMessageID, requestMissingMessages, eventsQueue) && solid
		}
	})
	if !solid {
		return false
	}

	if !s.parentsAgeValid(message.IssuingTime(), message.ParentsByType(StrongParentType)) {
		if len(message.ParentsByType(WeakParentType)) == 0 && messageMetadata.SetInvalid(true) {
			eventsQueue.Queue(s.tangle.Events.MessageInvalid, message.ID())
		}

		return false
	}

	message.ForEachParentByType(WeakParentType, func(parentMessageID MessageID) {
		if solid || requestMissingMessages {
			solid = s.isMessageMarkedAsWeaklySolid(parentMessageID, requestMissingMessages, eventsQueue) && solid
		}
	})
	if !solid {
		return false
	}

	if !s.parentsAgeValid(message.IssuingTime(), message.ParentsByType(WeakParentType)) {
		if messageMetadata.SetInvalid(true) {
			eventsQueue.Queue(s.tangle.Events.MessageInvalid, message.ID())
		}

		return false
	}

	return s.solidifyPayload(message, messageMetadata, eventsQueue)
}

// parentsAgeValid checks if the given MessageIDs belong to Messages that are within the allowed
// maxParentsTimeDifference from the issuing time.
func (s *Solidifier) parentsAgeValid(issuingTime time.Time, parentMessageIDs MessageIDs) (valid bool) {
	for _, parentMessageID := range parentMessageIDs {
		if parentMessageID == EmptyMessageID {
			continue
		}

		if !s.tangle.Storage.Message(parentMessageID).Consume(func(parentMessage *Message) {
			timeDifference := issuingTime.Sub(parentMessage.IssuingTime())

			valid = timeDifference >= minParentsTimeDifference && timeDifference <= maxParentsTimeDifference
		}) || !valid {
			return false
		}
	}

	return true
}

// isMessageMarkedAsSolid checks if the Message is already marked as solid (it requests missing messages).
func (s *Solidifier) isMessageMarkedAsSolid(messageID MessageID, requestMissingMessages bool, eventsQueue *eventsqueue.EventsQueue) (solid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if !requestMissingMessages {
			return nil
		}

		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID, StrongSolidificationSource)); stored {
			cachedMissingMessage.Consume(func(missingMessage *MissingMessage) {
				eventsQueue.Queue(s.Events.MessageMissing, messageID)
			})
		}

		return nil
	}).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid()
	})

	return
}

// isMessageMarkedAsSolid checks if the Message is already marked as weakly solid (it requests missing messages).
func (s *Solidifier) isMessageMarkedAsWeaklySolid(messageID MessageID, requestMissingMessages bool, eventsQueue *eventsqueue.EventsQueue) (solid bool) {
	if messageID == EmptyMessageID {
		return true
	}

	s.tangle.Storage.MessageMetadata(messageID, func() *MessageMetadata {
		if !requestMissingMessages {
			return nil
		}

		if cachedMissingMessage, stored := s.tangle.Storage.StoreMissingMessage(NewMissingMessage(messageID, WeakSolidificationSource)); stored {
			cachedMissingMessage.Consume(func(missingMessage *MissingMessage) {
				eventsQueue.Queue(s.Events.MessageMissing, messageID)
			})
		}

		return nil
	}).Consume(func(messageMetadata *MessageMetadata) {
		solid = messageMetadata.IsSolid() || messageMetadata.IsWeaklySolid()
	})

	return
}

// solidifyPayload creates the Attachment reference between the Message and its contained Transaction and then
// solidifies the Transaction by handing it over to the UTXODAG.
func (s *Solidifier) solidifyPayload(message *Message, messageMetadata *MessageMetadata, eventsQueue *eventsqueue.EventsQueue) (solid bool) {
	transaction, typeCastOkay := message.Payload().(*ledgerstate.Transaction)
	if !typeCastOkay {
		return true
	}

	if s.payloadSolidificationRunning(transaction.ID()) {
		return true
	}

	s.tangle.Storage.Attachment(transaction.ID(), message.ID(), NewAttachment).Release()

	s.setPayloadSolidificationRunning(transaction.ID(), true)
	_, solidityType, err := s.tangle.LedgerState.UTXODAG.StoreTransaction(transaction)
	s.setPayloadSolidificationRunning(transaction.ID(), false)
	if err != nil {
		switch {
		case errors.Is(err, ledgerstate.ErrInvalidStateTransition):
			messageMetadata.SetInvalid(true)
			eventsQueue.Queue(s.tangle.Events.MessageInvalid, message.ID())
		case errors.Is(err, ledgerstate.ErrTransactionInvalid):
			messageMetadata.SetInvalid(true)
			eventsQueue.Queue(s.tangle.Events.MessageInvalid, message.ID())
		default:
			eventsQueue.Queue(s.Events.Error, errors.Errorf("failed to store Transaction: %w", err))
		}

		return false
	}
	solid = solidityType == ledgerstate.Solid || solidityType == ledgerstate.LazySolid || solidityType == ledgerstate.Invalid

	return solid
}

// propagateSolidity executes the solidity checks for the given MessageIDs and propagates possible updates to their
// approvers.
func (s *Solidifier) propagateSolidity(messageIDs ...MessageID) {
	s.tangle.Utils.WalkMessageAndMetadata(func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker) {
		for _, messageID := range s.solidifyWeakly(message, messageMetadata, false) {
			walker.Push(messageID)
		}
	}, messageIDs, true)
}

// triggerWeaklySolidUpdate triggers the update of the MessageMetadata and the corresponding Event. It returns true if
// the Event was fired and false if the corresponding Message was already marked as weaklySolid, before.
func (s *Solidifier) triggerWeaklySolidUpdate(messageMetadata *MessageMetadata, eventsQueue *eventsqueue.EventsQueue) (triggered bool) {
	if !messageMetadata.SetWeaklySolid(true) {
		return false
	}

	eventsQueue.Queue(s.Events.MessageWeaklySolid, messageMetadata.ID())

	return true
}

// triggerSolidUpdate triggers the update of the MessageMetadata and the corresponding Event. It returns true if
// the Event was fired and false if the corresponding Message was already marked as solid, before.
func (s *Solidifier) triggerSolidUpdate(messageMetadata *MessageMetadata, eventsQueue *eventsqueue.EventsQueue) (triggered bool) {
	if !messageMetadata.SetSolid(true) {
		return false
	}

	eventsQueue.Queue(s.Events.MessageSolid, messageMetadata.ID())

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// Error is triggered when an unexpected error occurred.
	Error *events.Event

	// MessageWeaklySolid is triggered when a message becomes weakly solid, i.e. its weak references are solid and its
	// payload is solid.
	MessageWeaklySolid *events.Event

	// MessageSolid is triggered when a message becomes solid, i.e. all of its dependencies are known and solid.
	MessageSolid *events.Event

	// MessageMissing is triggered when a message references an unknown parent Message.
	MessageMissing *events.Event
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidificationType ///////////////////////////////////////////////////////////////////////////////////////////

// SolidificationType is a type that represents the different forms of solidification used in the Tangle.
type SolidificationType uint8

const (
	// UndefinedSolidificationType represents the zero value of a SolidificationType.
	UndefinedSolidificationType SolidificationType = iota

	// WeakSolidification represents the type of solidification where the node requests only the weak parents of a
	// Message.
	WeakSolidification

	// StrongSolidification represents the type of solidification where the node requests all parents of a Message.
	StrongSolidification
)

// SolidificationTypeFromBytes unmarshals a SolidificationType from a sequence of bytes.
func SolidificationTypeFromBytes(solidificationTypeBytes []byte) (solidificationType SolidificationType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(solidificationTypeBytes)
	if solidificationType, err = SolidificationTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SolidificationType from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SolidificationTypeFromMarshalUtil unmarshals a SolidificationType using a MarshalUtil (for easier unmarshaling).
func SolidificationTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (solidificationType SolidificationType, err error) {
	untypedSolidificationType, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse SolidificationType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return SolidificationType(untypedSolidificationType), nil
}

// Bytes returns a marshaled version of the SolidificationType.
func (s SolidificationType) Bytes() (marshaledSolidificationType []byte) {
	return []byte{uint8(s)}
}

// String returns a human-readable version of the SolidificationType.
func (s SolidificationType) String() (humanReadableSolidificationType string) {
	switch s {
	case UndefinedSolidificationType:
		return "SolidificationType(UndefinedSolidificationType)"
	case StrongSolidification:
		return "SolidificationType(StrongSolidification)"
	case WeakSolidification:
		return "SolidificationType(WeakSolidification)"
	default:
		return "SolidificationType(" + strconv.Itoa(int(s)) + ")"
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
