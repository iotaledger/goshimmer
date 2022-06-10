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
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

const storeSequenceInterval = 100

// region MessageFactory ///////////////////////////////////////////////////////////////////////////////////////////////

// MessageFactory acts as a factory to create new messages.
type MessageFactory struct {
	Events *MessageFactoryEvents

	tangle         *Tangle
	sequence       *kvstore.Sequence
	localIdentity  *identity.LocalIdentity
	selector       TipSelector
	referencesFunc ReferencesFunc

	powTimeout time.Duration

	worker      Worker
	workerMutex sync.RWMutex
}

// NewMessageFactory creates a new message factory.
func NewMessageFactory(tangle *Tangle, selector TipSelector, referencesFunc ReferencesFunc) *MessageFactory {
	sequence, err := kvstore.NewSequence(tangle.Options.Store, []byte(DBSequenceNumber), storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create message sequence number: %v", err))
	}

	return &MessageFactory{
		Events:         NewMessageFactoryEvents(),
		tangle:         tangle,
		sequence:       sequence,
		localIdentity:  tangle.Options.Identity,
		selector:       selector,
		referencesFunc: referencesFunc,
		worker:         ZeroWorker,
		powTimeout:     0 * time.Second,
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
	return f.issuePayload(p, nil, parentsCount...)
}

// IssuePayloadWithReferences creates a new message with the references submit.
func (f *MessageFactory) IssuePayloadWithReferences(p payload.Payload, references ParentMessageIDs) (*Message, error) {
	return f.issuePayload(p, references)
}

// issuePayload create a new message. If there are any supplied references, it uses them. Otherwise, uses tip selection.
// It also triggers the MessageConstructed event once it's done, which is for example used by the plugins to listen for
// messages that shall be attached to the tangle.
func (f *MessageFactory) issuePayload(p payload.Payload, references ParentMessageIDs, parentsCount ...int) (*Message, error) {
	payloadBytes, err := p.Bytes()
	if err != nil {
		err = errors.Errorf("could not serialize payload: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	payloadLen := len(payloadBytes)
	if payloadLen > payload.MaxSize {
		err := fmt.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	sequenceNumber, err := f.sequence.Next()
	if err != nil {
		err = errors.Errorf("could not create sequence number: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	issuerPublicKey := f.localIdentity.PublicKey()

	// do the PoW
	startTime := time.Now()
	var errPoW error
	var nonce uint64
	var issuingTime time.Time

	strongParents := references[StrongParentType]

	countParents := 2
	if len(parentsCount) > 0 {
		countParents = parentsCount[0]
	}

	fmt.Println(">> calling CommittmentFunc from MessageFactory")
	epochCommitment, lastConfirmedEpochIndex, epochCommitmentErr := f.tangle.Options.CommitmentFunc()
	if epochCommitmentErr != nil {
		err = errors.Errorf("cannot retrieve epoch commitment: %w", epochCommitmentErr)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	for run := true; run; run = errPoW != nil && time.Since(startTime) < f.powTimeout {
		_, txOk := p.(utxo.Transaction)
		if len(references) == 0 && (len(strongParents) == 0 || !txOk) {
			if strongParents, err = f.tips(p, countParents); err != nil {
				err = errors.Errorf("tips could not be selected: %w", err)
				f.Events.Error.Trigger(err)
				return nil, err
			}
		}
		issuingTime = f.getIssuingTime(strongParents)
		if len(references) == 0 {
			var referenceNotPossible MessageIDs
			references, referenceNotPossible, err = f.referencesFunc(p, strongParents, issuingTime, f.tangle)
			for m := range referenceNotPossible {
				f.Events.Error.Trigger(errors.Errorf("References for %s could not be determined", m))
				f.Events.MessageReferenceImpossible.Trigger(&MessageReferenceImpossibleEvent{m})
			}
			if err != nil {
				err = errors.Errorf("references could not be prepared: %w", err)
				f.Events.Error.Trigger(err)
				return nil, err
			}
		}
		nonce, errPoW = f.doPOW(references, issuingTime, issuerPublicKey, sequenceNumber, p, lastConfirmedEpochIndex, epochCommitment)
	}

	if errPoW != nil {
		err = errors.Errorf("pow failed: %w", errPoW)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// create the signature
	signature, err := f.sign(references, issuingTime, issuerPublicKey, sequenceNumber, p, nonce, lastConfirmedEpochIndex, epochCommitment)
	if err != nil {
		err = errors.Errorf("signing failed: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	msg, err := NewMessageWithValidation(
		references,
		issuingTime,
		issuerPublicKey,
		sequenceNumber,
		p,
		nonce,
		signature,
		lastConfirmedEpochIndex,
		epochCommitment,
	)
	if err != nil {
		err = errors.Errorf("there is a problem with the message syntax: %w", err)
		f.Events.Error.Trigger(err)
		return nil, err
	}
	_ = msg.DetermineID()

	f.Events.MessageConstructed.Trigger(&MessageConstructedEvent{msg})
	return msg, nil
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

func (f *MessageFactory) tips(p payload.Payload, parentsCount int) (parents MessageIDs, err error) {
	parents, err = f.selector.Tips(p, parentsCount)

	tx, ok := p.(utxo.Transaction)
	if ok {
		conflictingTransactions := f.tangle.Ledger.Utils.ConflictingTransactions(tx.ID())
		if !conflictingTransactions.IsEmpty() {
			switch earliestAttachment := f.EarliestAttachment(conflictingTransactions); earliestAttachment {
			case nil:
				return
			default:
				return earliestAttachment.ParentsByType(StrongParentType), nil
			}
		}
	}

	return
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
func (f *MessageFactory) doPOW(references ParentMessageIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload, latestConfirmedEpoch epoch.EI, epochCommitment *epoch.ECRecord) (uint64, error) {
	// create a dummy message to simplify marshaling
	message := NewMessage(references, issuingTime, key, seq, messagePayload, 0, ed25519.EmptySignature, latestConfirmedEpoch, epochCommitment)
	dummy, err := message.Bytes()
	if err != nil {
		return 0, err
	}

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *MessageFactory) sign(references ParentMessageIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, messagePayload payload.Payload, nonce uint64, latestConfirmedEpoch epoch.EI, epochCommitment *epoch.ECRecord) (ed25519.Signature, error) {
	// create a dummy message to simplify marshaling
	dummy := NewMessage(references, issuingTime, key, seq, messagePayload, nonce, ed25519.EmptySignature, latestConfirmedEpoch, epochCommitment)
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

// ReferencesFunc is a function type that returns like references a given set of parents of a Message.
type ReferencesFunc func(payload payload.Payload, strongParents MessageIDs, issuingTime time.Time, tangle *Tangle) (references ParentMessageIDs, referenceNotPossible MessageIDs, err error)

// PrepareReferences is an implementation of ReferencesFunc.
func PrepareReferences(payload payload.Payload, strongParents MessageIDs, issuingTime time.Time, tangle *Tangle) (references ParentMessageIDs, referenceNotPossible MessageIDs, err error) {
	references = NewParentMessageIDs()
	referenceNotPossible = NewMessageIDs()

	for strongParent := range strongParents {
		if strongParent == EmptyMessageID {
			references.AddStrong(strongParent)
			continue
		}

		strongParentBranchIDs, err := tangle.Booker.MessageBranchIDs(strongParent)

		if strongParentBranchIDs.Size() == 0 {
			references.AddStrong(strongParent)
			continue
		}

		if err != nil {
			return nil, referenceNotPossible, errors.Errorf("branchID for Parent with %s can't be retrieved: %w", strongParent, err)
		}

		referencesCopy := references.Clone()

		opinionCanBeExpressed := true
		for it := strongParentBranchIDs.Iterator(); it.HasNext(); {
			referenceParentType, referenceMessageID, err := referenceFromStrongParent(tangle, it.Next(), issuingTime)
			// Explicitly ignore error since we can't create a like/dislike reference to the message.
			// This means this message can't be added as a strong parent.
			if err != nil {
				opinionCanBeExpressed = false
				break
			}

			if referenceParentType == UndefinedParentType {
				continue
			}

			references.Add(referenceParentType, referenceMessageID)

			if len(references[referenceParentType]) > MaxParentsCount {
				opinionCanBeExpressed = false
				break
			}
		}

		if !opinionCanBeExpressed {
			// If the opinion cannot be expressed, we have to rollback to the previous reference collection.
			references = referencesCopy
			strongParentPayloadBranchIDs, strongParentPayloadBranchIDsErr := tangle.Booker.PayloadBranchIDs(strongParent)
			if strongParentPayloadBranchIDsErr != nil {
				return nil, referenceNotPossible, errors.Errorf("failed to determine payload branch ids of strong parent with %s: %w", strongParent, strongParentPayloadBranchIDsErr)
			}

			if tangle.Utils.AllBranchesLiked(strongParentPayloadBranchIDs) {
				references.Add(WeakParentType, strongParent)
			}
			// If we can't add express the opinion (can't reference past cone), the message needs to be removed from the tip pool.
			referenceNotPossible.Add(strongParent)
			continue
		}

		references.AddStrong(strongParent)
	}

	if len(references[StrongParentType]) == 0 {
		return nil, referenceNotPossible, errors.Errorf("none of the provided strong parents can be referenced")
	}

	for _, parentType := range []ParentsType{StrongParentType, WeakParentType, ShallowLikeParentType, ShallowDislikeParentType} {
		if len(references[parentType]) > MaxParentsCount {
			return nil, referenceNotPossible, errors.Errorf("parent overflow while creating references for message with strong Parents %s", strongParents)
		}

	}

	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
	if tx, isTx := payload.(utxo.Transaction); isTx {
		referencedTxs := tangle.Ledger.Utils.ReferencedTransactions(tx)
		for it := referencedTxs.Iterator(); it.HasNext(); {
			referencedTx := it.Next()
			if !tangle.ConfirmationOracle.IsTransactionConfirmed(referencedTx) {
				latestAttachment := tangle.MessageFactory.LatestAttachment(referencedTx)
				if latestAttachment == nil {
					continue
				}
				timeDifference := clock.SyncedTime().Sub(latestAttachment.IssuingTime())
				// If the latest attachment of the transaction we are consuming is too old we are not
				// able to add it is a weak parent.
				if timeDifference <= maxParentsTimeDifference {
					if len(references[WeakParentType]) == MaxParentsCount {
						return references, referenceNotPossible, nil
					}
					references.Add(WeakParentType, latestAttachment.ID())
				}
			}
		}
	}

	return references, referenceNotPossible, nil
}

func referenceFromStrongParent(tangle *Tangle, strongParentBranchID utxo.TransactionID, issuingTime time.Time) (parentType ParentsType, reference MessageID, err error) {
	likedBranchID, conflictMembers := tangle.OTVConsensusManager.LikedConflictMember(strongParentBranchID)
	if likedBranchID == strongParentBranchID {
		return
	}

	if likedBranchID != utxo.EmptyTransactionID {
		txID := likedBranchID
		oldestAttachmentTime, oldestAttachmentMessageID, err := tangle.Utils.FirstAttachment(txID)
		if err != nil {
			return UndefinedParentType, EmptyMessageID, errors.Errorf("failed to find first attachment of Transaction with %s: %w", txID, err)
		}
		if issuingTime.Sub(oldestAttachmentTime) >= maxParentsTimeDifference {
			return UndefinedParentType, EmptyMessageID, errors.Errorf("shallow like reference needed for Transaction with %s is too far in the past", txID)
		}

		return ShallowLikeParentType, oldestAttachmentMessageID, nil
	}

	for it := conflictMembers.Iterator(); it.HasNext(); {
		conflictMember := it.Next()

		// Always point to another branch, to make sure the receiver forks the branch.
		if conflictMember == strongParentBranchID {
			continue
		}

		oldestAttachmentTime, oldestAttachmentMessageID, err := tangle.Utils.FirstAttachment(conflictMember)
		if err != nil {
			return UndefinedParentType, EmptyMessageID, errors.Errorf("failed to find first attachment of Transaction with %s: %w", conflictMember, err)
		}

		if issuingTime.Sub(oldestAttachmentTime) < maxParentsTimeDifference {
			return ShallowDislikeParentType, oldestAttachmentMessageID, nil
		}
	}

	return UndefinedParentType, EmptyMessageID, errors.Errorf("shallow dislike reference needed for Transaction with %s is too far in the past", strongParentBranchID)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
