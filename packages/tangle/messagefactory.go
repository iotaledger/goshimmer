package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const storeSequenceInterval = 100

// region MessageFactory ///////////////////////////////////////////////////////////////////////////////////////////////

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events *MessageFactoryEvents

	tangle             *Tangle
	sequence           *kvstore.Sequence
	localIdentity      *identity.LocalIdentity
	selector           TipSelector
	likeReferencesFunc LikeReferencesFunc

	powTimeout time.Duration

	worker        Worker
	workerMutex   sync.RWMutex
	issuanceMutex sync.Mutex
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(tangle *Tangle, selector TipSelector, likeReferences LikeReferencesFunc) *MessageFactory {
	sequence, err := kvstore.NewSequence(tangle.Options.Store, []byte(DBSequenceNumber), storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events: &MessageFactoryEvents{
			MessageConstructed: events.NewEvent(messageEventHandler),
			Error:              events.NewEvent(events.ErrorCaller),
		},

		tangle:             tangle,
		sequence:           sequence,
		localIdentity:      tangle.Options.Identity,
		selector:           selector,
		likeReferencesFunc: likeReferences,
		worker:             ZeroWorker,
		powTimeout:         0 * time.Second,
	}
}

// SetWorker sets the PoW worker to be used for the messages.
func (f *MessageFactory) SetWorker(worker Worker) {
	f.workerMutex.Lock()
	defer f.workerMutex.Unlock()
	f.worker = worker
}

// SetTimeout sets the timeout for PoW.
func (f *MessageFactory) SetTimeout(timeout time.Duration) {
	f.powTimeout = timeout
}

// IssuePayload creates a new message including sequence number and tip selection and returns it.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (f *MessageFactory) IssuePayload(p payload.Payload, parentsCount ...int) (*Message, error) {
	payloadLen := len(p.Bytes())
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	f.issuanceMutex.Lock()

	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = errors.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		f.issuanceMutex.Unlock()
		return nil, err
	}

	countParents := 2
	if len(parentsCount) > 0 {
		countParents = parentsCount[0]
	}

	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	startTime := time.Now()
	var errPoW error
	var nonce uint64
	var parents MessageIDs
	var issuingTime time.Time
	var likeReferences MessageIDs

	for run := true; run; run = errPoW != nil && time.Since(startTime) < f.powTimeout {
		if len(parents) == 0 || p.Type() != ledgerstate.TransactionType {
			if parents, err = f.tips(p, countParents); err != nil {
				err = errors.Errorf("tips could not be selected: %w", err)
				f.Events.Error.Trigger(err)
				f.issuanceMutex.Unlock()
				return nil, err
			}
		}
		issuingTime = f.getIssuingTime(parents)

		likeReferences, err = f.likeReferencesFunc(parents, issuingTime, f.tangle)
		if err != nil {
			err = errors.Errorf("like references could not be prepared: %w", err)
			f.Events.Error.Trigger(err)
			f.issuanceMutex.Unlock()
			return nil, err
		}
		nonce, errPoW = f.doPOW(parents, nil, likeReferences, issuingTime, issuerPublicKey, sequenceNumber, p)
	}

	if errPoW != nil {
		err = errors.Errorf("pow failed: %w", errPoW)
		f.Events.Error.Trigger(err)
		f.issuanceMutex.Unlock()
		return nil, err
	}
	f.issuanceMutex.Unlock()

	// create the signature
	signature, err := f.sign(parents, nil, likeReferences, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)
	if err != nil {
		err = errors.Errorf("signing failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	msg, err := NewMessage(
		parents,
		nil,
		nil,
		likeReferences,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	if err != nil {
		err = errors.Errorf("there is a problem with the message syntax: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	f.Events.MessageConstructed.Trigger(msg)
	return msg, nil
}

func (f *MessageFactory) getIssuingTime(parents MessageIDs) time.Time {
	issuingTime := clock.SyncedTime()

	// due to the ParentAge check we must ensure that we set the right issuing time.

	for _, parent := range parents {
		f.tangle.Storage.Message(parent).Consume(func(msg *Message) {
			if msg.ID() != EmptyMessageID && !msg.IssuingTime().Before(issuingTime) {
				issuingTime = msg.IssuingTime()
			}
		})
	}

	return issuingTime
}

func (f *MessageFactory) tips(p payload.Payload, parentsCount int) (parents MessageIDs, err error) {
	parents, err = f.selector.Tips(p, parentsCount)

	if p.Type() == ledgerstate.TransactionType {
		conflictingTransactions := f.tangle.LedgerState.UTXODAG.ConflictingTransactions(p.(*ledgerstate.Transaction))
		if len(conflictingTransactions) != 0 {
			switch earliestAttachment := f.earliestAttachment(conflictingTransactions); earliestAttachment {
			case nil:
				return
			default:
				return earliestAttachment.ParentsByType(StrongParentType), nil
			}
		}
	}

	return
}

func (f *MessageFactory) earliestAttachment(transactionIDs ledgerstate.TransactionIDs) (earliestAttachment *Message) {
	earliestIssuingTime := time.Now()
	for transactionID := range transactionIDs {
		f.tangle.Storage.Attachments(transactionID).Consume(func(attachment *Attachment) {
			f.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
				f.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
					if messageMetadata.IsBooked() && message.IssuingTime().Before(earliestIssuingTime) {
						earliestAttachment = message
					}
				})
			})
		})
	}

	return earliestAttachment
}

// Shutdown closes the MessageFactory and persists the sequence number.
func (f *MessageFactory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

func (f *MessageFactory) doPOW(strongParents, weakParents, likeParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	message, err := NewMessage(strongParents, weakParents, nil, likeParents, issuingTime, key, seq, messagePayload, 0, ed25519.EmptySignature)
	if err != nil {
		return 0, err
	}

	dummy := message.Bytes()

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *MessageFactory) sign(strongParents, weakParents, likeParents []MessageID, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload, nonce uint64) (ed25519.Signature, error) {
	// create a dummy message to simplify marshaling
	dummy, err := NewMessage(strongParents, weakParents, nil, likeParents, issuingTime, key, seq, messagePayload, nonce, ed25519.EmptySignature)
	if err != nil {
		return ed25519.EmptySignature, err
	}
	dummyBytes := dummy.Bytes()

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength]), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageFactoryEvents /////////////////////////////////////////////////////////////////////////////////////////

// MessageFactoryEvents represents events happening on a message factory.
type MessageFactoryEvents struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *events.Event

	// Fired when an error occurred.
	Error *events.Event
}

func messageEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*Message))(params[0].(*Message))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelector //////////////////////////////////////////////////////////////////////////////////////////////////

// A TipSelector selects two tips, parent2 and parent1, for a new message to attach to.
type TipSelector interface {
	Tips(p payload.Payload, countParents int) (parents MessageIDs, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelectorFunc //////////////////////////////////////////////////////////////////////////////////////////////

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(p payload.Payload, countParents int) (parents MessageIDs, err error)

// Tips calls f().
func (f TipSelectorFunc) Tips(p payload.Payload, countParents int) (parents MessageIDs, err error) {
	return f(p, countParents)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Worker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// A Worker performs the PoW for the provided message in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WorkerFunc ///////////////////////////////////////////////////////////////////////////////////////////////////

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(msg).
func (f WorkerFunc) DoPOW(msg []byte) (uint64, error) {
	return f(msg)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ZeroWorker ///////////////////////////////////////////////////////////////////////////////////////////////////

// ZeroWorker is a PoW worker that always returns 0 as the nonce.
var ZeroWorker = WorkerFunc(func([]byte) (uint64, error) { return 0, nil })

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PrepareLikeReferences ///////////////////////////////////////////////////////////////////////////////////////////////////

// LikeReferencesFunc is a function type that returns like references a given set of parents of a Message.
type LikeReferencesFunc func(parents MessageIDs, issuingTime time.Time, tangle *Tangle) (MessageIDs, error)

// PrepareLikeReferences is an implementation of LikeReferencesFunc.
func PrepareLikeReferences(parents MessageIDs, issuingTime time.Time, tangle *Tangle) (MessageIDs, error) {
	branchIDs := ledgerstate.NewBranchIDs()

	for _, parent := range parents {
		branchID, err := tangle.Booker.MessageBranchID(parent)
		if err != nil {
			err = errors.Errorf("branchID can't be retrieved: %w", err)
			return nil, err
		}
		branchIDs.Add(branchID)
	}
	_, dislikedBranches, err := tangle.OTVConsensusManager.Opinion(branchIDs)
	if err != nil {
		err = errors.Errorf("opinions could not be retrieved: %w", err)
		return nil, err
	}
	likeReferencesMap := make(map[MessageID]types.Empty)
	likeReferences := MessageIDs{}

	for dislikedBranch := range dislikedBranches {
		likedInstead, err := tangle.OTVConsensusManager.LikedInstead(dislikedBranch)
		if err != nil {
			err = errors.Errorf("branch liked instead could not be retrieved: %w", err)
			return nil, err
		}

		for _, likeRef := range likedInstead {
			oldestAttachmentTime, oldestAttachmentMessageID, err := tangle.Utils.FirstAttachment(likeRef.Liked.TransactionID())
			if err != nil {
				return nil, err
			}
			// add like reference to a message only once if it appears in multiple conflict sets
			if _, ok := likeReferencesMap[oldestAttachmentMessageID]; !ok {
				likeReferencesMap[oldestAttachmentMessageID] = types.Void
				// check difference between issuing time and message that would be set as like reference, to avoid setting too old message.
				// what if original message is older than maxParentsTimeDifference even though the branch still exists?
				if issuingTime.Sub(oldestAttachmentTime) < maxParentsTimeDifference {
					likeReferences = append(likeReferences, oldestAttachmentMessageID)
				}
			}
		}
	}
	return likeReferences, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
