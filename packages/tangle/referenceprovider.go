package tangle

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////

// ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
type ReferenceProvider struct {
	Events *ReferenceProviderEvents

	tangle *Tangle
}

// NewReferenceProvider creates a new ReferenceProvider instance.
func NewReferenceProvider(tangle *Tangle) (newInstance *ReferenceProvider) {
	return &ReferenceProvider{
		Events: newReferenceProviderEvents(),
		tangle: tangle,
	}
}

// References is an implementation of ReferencesFunc.
func (r *ReferenceProvider) References(payload payload.Payload, strongParents MessageIDs, issuingTime time.Time) (references ParentMessageIDs, err error) {
	references = NewParentMessageIDs()

	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
	if tx, isTx := payload.(utxo.Transaction); isTx {
		referencedTxs := r.tangle.Ledger.Utils.ReferencedTransactions(tx)
		for it := referencedTxs.Iterator(); it.HasNext(); {
			referencedTx := it.Next()
			if !r.tangle.ConfirmationOracle.IsTransactionConfirmed(referencedTx) {
				latestAttachment := r.tangle.MessageFactory.LatestAttachment(referencedTx)
				if latestAttachment == nil {
					continue
				}
				timeDifference := clock.SyncedTime().Sub(latestAttachment.IssuingTime())
				// If the latest attachment of the transaction we are consuming is too old we are not
				// able to add it is a weak parent.
				if timeDifference <= maxParentsTimeDifference {
					if len(references[WeakParentType]) == MaxParentsCount {
						return references, nil
					}
					references.Add(WeakParentType, latestAttachment.ID())
				}
			}
		}
	}

	for strongParent := range strongParents {
		referencesToAdd, validStrongParent := r.addedReferencesForMessage(strongParent, issuingTime)
		if !validStrongParent {
			if err = r.checkPayloadLiked(strongParent); err != nil {
				r.Events.Error.Trigger(errors.Errorf("failed to pick up %s as a weak parent: %w", strongParent, err))
				continue
			}
			referencesToAdd = NewParentMessageIDs().Add(WeakParentType, strongParent)
		} else {
			referencesToAdd.AddStrong(strongParent)
		}

		if combinedReferences, success := r.tryExtendReferences(references, referencesToAdd); success {
			references = combinedReferences
		}
	}

	if len(references[StrongParentType]) == 0 {
		return nil, errors.Errorf("none of the provided strong parents can be referenced")
	}

	return references, nil
}

// addedReferenceForMessage returns the reference that is necessary to correct our opinion on the given message.
func (r *ReferenceProvider) addedReferencesForMessage(msgID MessageID, issuingTime time.Time) (addedReferences ParentMessageIDs, success bool) {
	msgConflictIDs, err := r.tangle.Booker.MessageBranchIDs(msgID)
	if err != nil {
		r.Events.Error.Trigger(errors.Errorf("conflictID of %s can't be retrieved: %w", msgID, err))
		r.Events.ReferenceImpossible.Trigger(msgID)
		return nil, false
	}

	addedReferences = NewParentMessageIDs()
	if msgConflictIDs.IsEmpty() {
		return addedReferences, true
	}

	if addedReferences, err = r.addedReferencesForConflicts(msgConflictIDs, issuingTime); err != nil {
		r.Events.Error.Trigger(errors.Errorf("cannot pick up %s as strong parent: %w", msgID, err))
		r.Events.ReferenceImpossible.Trigger(msgID)
		return nil, false
	}

	fmt.Println("addedReferencesForMessage", msgID, addedReferences)
	// A message might introduce too many references and cannot be picked up as a strong parent.
	if _, success := r.tryExtendReferences(NewParentMessageIDs(), addedReferences); !success {
		fmt.Println("too many references for", msgID)
		r.Events.Error.Trigger(errors.Errorf("cannot pick up %s as strong parent: %w", msgID, err))
		r.Events.ReferenceImpossible.Trigger(msgID)
		return nil, false
	}

	return addedReferences, true
}

// addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, issuingTime time.Time) (referencesToAdd ParentMessageIDs, err error) {
	referencesToAdd = NewParentMessageIDs()
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if adjust, referencedMsg, referenceErr := r.adjustOpinion(conflictID, issuingTime); referenceErr != nil {
			return nil, errors.Errorf("failed to create reference for %s: %w", conflictID, referenceErr)
		} else if adjust {
			referencesToAdd.Add(ShallowLikeParentType, referencedMsg)
		}
	}

	return referencesToAdd, nil
}

// adjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, issuingTime time.Time) (adjust bool, msgID MessageID, err error) {
	for w := walker.New[utxo.TransactionID](false).Push(conflictID); w.HasNext(); {
		currentConflictID := w.Next()

		likedConflictID, _ := r.tangle.OTVConsensusManager.LikedConflictMember(currentConflictID)
		// only triggers in first iteration
		if likedConflictID == conflictID {
			return false, EmptyMessageID, nil
		}

		if !likedConflictID.IsEmpty() {
			if msgID, err = r.firstValidAttachment(currentConflictID, issuingTime); err != nil {
				continue
			}

			return true, msgID, nil
		}

		// only walk deeper if we don't like "something else"
		r.tangle.Ledger.ConflictDAG.Storage.CachedConflict(currentConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			w.PushFront(conflict.Parents().Slice()...)
		})
	}

	return false, EmptyMessageID, errors.Newf("failed to create dislike for %s", conflictID)
}

// firstValidAttachment returns the first valid attachment of the given transaction.
func (r *ReferenceProvider) firstValidAttachment(txID utxo.TransactionID, issuingTime time.Time) (msgID MessageID, err error) {
	attachmentTime, msgID, err := r.tangle.Utils.FirstAttachment(txID)
	if err != nil {
		return EmptyMessageID, errors.Errorf("failed to find first attachment of Transaction with %s: %w", txID, err)
	}

	// TODO: we don't want to vote on anything that is in a committed epoch.
	if !r.validTime(attachmentTime, issuingTime) {
		return EmptyMessageID, errors.Errorf("attachment of %s with %s is too far in the past", txID, msgID)
	}

	return msgID, nil
}

func (r *ReferenceProvider) validTime(parentTime, issuingTime time.Time) bool {
	// TODO: we don't want to vote on anything that is in a committed epoch.
	return issuingTime.Sub(parentTime) < maxParentsTimeDifference
}

func (r *ReferenceProvider) validTimeForStrongParent(msgID MessageID, issuingTime time.Time) (valid bool) {
	r.tangle.Storage.Message(msgID).Consume(func(msg *Message) {
		valid = r.validTime(msg.IssuingTime(), issuingTime)
	})
	return valid
}

// checkPayloadLiked checks if the payload of a Message is liked.
func (r *ReferenceProvider) checkPayloadLiked(msgID MessageID) (err error) {
	conflictIDs, err := r.tangle.Booker.PayloadBranchIDs(msgID)
	if err != nil {
		return errors.Errorf("failed to determine payload conflictIDs of %s: %w", msgID, err)
	}

	if !r.tangle.Utils.AllBranchesLiked(conflictIDs) {
		return errors.Errorf("payload of %s is not liked: %s", msgID, conflictIDs)
	}

	return nil
}

// tryExtendReferences tries to extend the references with the given referencesToAdd.
func (r *ReferenceProvider) tryExtendReferences(references ParentMessageIDs, referencesToAdd ParentMessageIDs) (extendedReferences ParentMessageIDs, success bool) {
	if referencesToAdd.IsEmpty() {
		return references, true
	}

	extendedReferences = references.Clone()
	for referenceType, referencedMessageIDs := range referencesToAdd {
		extendedReferences.AddAll(referenceType, referencedMessageIDs)

		if len(extendedReferences[referenceType]) > MaxParentsCount {
			return nil, false
		}
	}

	return extendedReferences, true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferenceProviderEvents //////////////////////////////////////////////////////////////////////////////////////

type ReferenceProviderEvents struct {
	ReferenceImpossible *event.Event[MessageID]
	Error               *event.Event[error]
}

func newReferenceProviderEvents() (newInstance *ReferenceProviderEvents) {
	return &ReferenceProviderEvents{
		ReferenceImpossible: event.New[MessageID](),
		Error:               event.New[error](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
