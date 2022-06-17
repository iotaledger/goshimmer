package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const storeSequenceInterval = 100

// region MessageFactory ///////////////////////////////////////////////////////////////////////////////////////////////

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events *MessageFactoryEvents

	tangle            *Tangle
	sequence          *kvstore.Sequence
	localIdentity     *identity.LocalIdentity
	selector          TipSelector
	referencesFunc    ReferencesFunc
	ReferenceProvider *ReferenceProvider

	powTimeout time.Duration

	worker      Worker
	workerMutex sync.RWMutex
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(tangle *Tangle, selector TipSelector, referencesFunc ...ReferencesFunc) *MessageFactory {
	sequence, err := kvstore.NewSequence(tangle.Options.Store, []byte(DBSequenceNumber), storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	referenceProvider := NewReferenceProvider(tangle)
	f := referenceProvider.References
	if len(referencesFunc) != 0 {
		f = referencesFunc[0]
	}

	return &MessageFactory{
		Events:            NewMessageFactoryEvents(),
		tangle:            tangle,
		sequence:          sequence,
		localIdentity:     tangle.Options.Identity,
		selector:          selector,
		referencesFunc:    f,
		ReferenceProvider: referenceProvider,
		worker:            ZeroWorker,
		powTimeout:        0 * time.Second,
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
func (f *MessageFactory) IssuePayload(p payload.Payload, parentsCount ...int) (*Message, error) {
	msg, err := f.issuePayload(p, nil, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("message could not be issued: %w", err))
		return nil, err
	}

	f.Events.MessageConstructed.Trigger(&MessageConstructedEvent{msg})
	return msg, nil
}

// IssuePayloadWithReferences creates a new message with the references submit.
func (f *MessageFactory) IssuePayloadWithReferences(p payload.Payload, references ParentMessageIDs, parentsCount ...int) (*Message, error) {
	msg, err := f.issuePayload(p, references, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("message could not be issued: %w", err))
		return nil, err
	}

	f.Events.MessageConstructed.Trigger(&MessageConstructedEvent{msg})
	return msg, nil
}

// issuePayload create a new message. If there are any supplied references, it uses them. Otherwise, uses tip selection.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (f *MessageFactory) issuePayload(p payload.Payload, references ParentMessageIDs, parentsCountOpt ...int) (*Message, error) {
	parentsCount := 2
	if len(parentsCountOpt) > 0 {
		parentsCount = parentsCountOpt[0]
	}

	payloadBytes, err := p.Bytes()
	if err != nil {
		return nil, errors.Errorf("could not serialize payload: %w", err)
	}

	payloadLen := len(payloadBytes)
	if payloadLen > payload.MaxSize {
		return nil, errors.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
	}

	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		return nil, errors.Errorf("could not create sequence number: %w", err)
	}

	issuerPublicKey := f.localIdentity.PublicKey()

	// select tips, perform PoW and prepare references
	references, nonce, issuingTime, err := f.selectTipsAndPerformPoW(p, references, parentsCount, issuerPublicKey, sequenceNumber)
	if err != nil {
		return nil, errors.Errorf("could not select tips and perform PoW: %w", err)
	}

	// create the signature
	signature, err := f.sign(references, issuingTime, issuerPublicKey, sequenceNumber, p, nonce)
	if err != nil {
		return nil, errors.Errorf("signing failed: %w", err)
	}

	msg, err := NewMessageWithValidation(
		references,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
	)
	if err != nil {
		return nil, errors.Errorf("there is a problem with the message syntax: %w", err)
	}
	_ = msg.DetermineID()

	return msg, nil
}

func (f *MessageFactory) selectTipsAndPerformPoW(p payload.Payload, providedReferences ParentMessageIDs, parentsCount int, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64) (references ParentMessageIDs, nonce uint64, issuingTime time.Time, err error) {
	// Perform PoW with given information if there are references provided.
	if !providedReferences.IsEmpty() {
		issuingTime = f.getIssuingTime(providedReferences[StrongParentType])
		nonce, err = f.doPOW(providedReferences, issuingTime, issuerPublicKey, sequenceNumber, p)
		if err != nil {
			return providedReferences, nonce, issuingTime, errors.Errorf("PoW failed: %w", err)
		}
		return providedReferences, nonce, issuingTime, nil
	}

	// TODO: once we get rid of PoW we need to set another timeout here that allows to specify for how long we try to select tips if there are no valid references.
	//   This in turn should remove the invalid references from the tips bit by bit until there are valid strong parents again.
	startTime := time.Now()
	for run := true; run; run = err != nil && time.Since(startTime) < f.powTimeout {
		strongParents := f.tips(p, parentsCount)
		issuingTime = f.getIssuingTime(strongParents)
		references, err = f.referencesFunc(p, strongParents, issuingTime)
		// If none of the strong parents are possible references, we have to try again.
		if err != nil {
			f.Events.Error.Trigger(errors.Errorf("references could not be created: %w", err))
			continue
		}

		// Make sure that there's no duplicate between strong and weak parents.
		for strongParent := range references[StrongParentType] {
			delete(references[WeakParentType], strongParent)
		}

		// fill up weak references with weak references to liked missing branches
		if _, exists := references[WeakParentType]; !exists {
			references[WeakParentType] = NewMessageIDs()
		}
		references[WeakParentType].AddAll(f.ReferenceProvider.ReferencesToMissingConflicts(issuingTime, MaxParentsCount-len(references[WeakParentType])))

		if len(references[WeakParentType]) == 0 {
			delete(references, WeakParentType)
		}

		nonce, err = f.doPOW(references, issuingTime, issuerPublicKey, sequenceNumber, p)
	}

	if err != nil {
		return nil, 0, time.Time{}, errors.Errorf("pow failed: %w", err)
	}

	return references, nonce, issuingTime, nil
}

func (f *MessageFactory) getIssuingTime(parents MessageIDs) time.Time {
	issuingTime := clock.SyncedTime()

	// due to the ParentAge check we must ensure that we set the right issuing time.

	for parent := range parents {
		f.tangle.Storage.Message(parent).Consume(func(msg *Message) {
			if msg.ID() != EmptyMessageID && !msg.IssuingTime().Before(issuingTime) {
				issuingTime = msg.IssuingTime()
			}
		})
	}

	return issuingTime
}

func (f *MessageFactory) tips(p payload.Payload, parentsCount int) (parents MessageIDs) {
	parents = f.selector.Tips(p, parentsCount)

	tx, ok := p.(utxo.Transaction)
	if !ok {
		return parents
	}

	// If the message is issuing a transaction and is a double spend, we add it in parallel to the earliest attachment
	// to prevent a double spend from being issued in its past cone.
	if conflictingTransactions := f.tangle.Ledger.Utils.ConflictingTransactions(tx.ID()); !conflictingTransactions.IsEmpty() {
		if earliestAttachment := f.EarliestAttachment(conflictingTransactions); earliestAttachment != nil {
			return earliestAttachment.ParentsByType(StrongParentType)
		}
	}

	return parents
}

func (f *MessageFactory) EarliestAttachment(transactionIDs utxo.TransactionIDs) (earliestAttachment *Message) {
	earliestIssuingTime := time.Now()
	for it := transactionIDs.Iterator(); it.HasNext(); {
		f.tangle.Storage.Attachments(it.Next()).Consume(func(attachment *Attachment) {
			f.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
				f.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
					if messageMetadata.IsBooked() && message.IssuingTime().Before(earliestIssuingTime) {
						earliestAttachment = message
						earliestIssuingTime = message.IssuingTime()
					}
				})
			})
		})
	}

	return earliestAttachment
}

func (f *MessageFactory) LatestAttachment(transactionID utxo.TransactionID) (latestAttachment *Message) {
	var latestIssuingTime time.Time
	f.tangle.Storage.Attachments(transactionID).Consume(func(attachment *Attachment) {
		f.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
			f.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
				if messageMetadata.IsBooked() && message.IssuingTime().After(latestIssuingTime) {
					latestAttachment = message
					latestIssuingTime = message.IssuingTime()
				}
			})
		})
	})

	return latestAttachment
}

// Shutdown closes the MessageFactory and persists the sequence number.
func (f *MessageFactory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release message sequence number: %w", err))
	}
}

// doPOW performs pow on the message and returns a nonce.
func (f *MessageFactory) doPOW(references ParentMessageIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload) (uint64, error) {
	// create a dummy message to simplify marshaling
	message := NewMessage(references, issuingTime, key, seq, messagePayload, 0, ed25519.EmptySignature)
	dummy, err := message.Bytes()
	if err != nil {
		return 0, err
	}

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *MessageFactory) sign(references ParentMessageIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload, nonce uint64) (ed25519.Signature, error) {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(references, issuingTime, key, seq, messagePayload, nonce, ed25519.EmptySignature)
	dummyBytes, err := dummy.Bytes()
	if err != nil {
		return ed25519.EmptySignature, err
	}

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength]), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelector //////////////////////////////////////////////////////////////////////////////////////////////////

// A TipSelector selects two tips, parent2 and parent1, for a new message to attach to.
type TipSelector interface {
	Tips(p payload.Payload, countParents int) (parents MessageIDs)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelectorFunc //////////////////////////////////////////////////////////////////////////////////////////////

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(p payload.Payload, countParents int) (parents MessageIDs)

// Tips calls f().
func (f TipSelectorFunc) Tips(p payload.Payload, countParents int) (parents MessageIDs) {
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

// ReferencesFunc is a function type that returns like references a given set of parents of a Message.
type ReferencesFunc func(payload payload.Payload, strongParents MessageIDs, issuingTime time.Time) (references ParentMessageIDs, err error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
