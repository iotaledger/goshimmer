package tangleold

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/node/clock"
)

// region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////

// ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
type ReferenceProvider struct {
	tangle *Tangle
}

// NewReferenceProvider creates a new ReferenceProvider instance.
func NewReferenceProvider(tangle *Tangle) (newInstance *ReferenceProvider) {
	return &ReferenceProvider{
		tangle: tangle,
	}
}

// References is an implementation of ReferencesFunc.
func (r *ReferenceProvider) References(payload payload.Payload, strongParents BlockIDs, issuingTime time.Time) (references ParentBlockIDs, err error) {
	references = NewParentBlockIDs()

	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
	if tx, isTx := payload.(utxo.Transaction); isTx {
		referencedTxs := r.tangle.Ledger.Utils.ReferencedTransactions(tx)
		for it := referencedTxs.Iterator(); it.HasNext(); {
			referencedTx := it.Next()
			if !r.tangle.ConfirmationOracle.IsTransactionConfirmed(referencedTx) {
				latestAttachment := r.tangle.BlockFactory.LatestAttachment(referencedTx)
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

	excludedConflictIDs := utxo.NewTransactionIDs()

	for strongParent := range strongParents {
		excludedConflictIDsCopy := excludedConflictIDs.Clone()
		referencesToAdd, validStrongParent := r.addedReferencesForBlock(strongParent, issuingTime, excludedConflictIDsCopy)
		if !validStrongParent {
			if err = r.checkPayloadLiked(strongParent); err != nil {
				continue
			}

			referencesToAdd = NewParentBlockIDs().Add(WeakParentType, strongParent)
		} else {
			referencesToAdd.AddStrong(strongParent)
		}

		if combinedReferences, success := r.tryExtendReferences(references, referencesToAdd); success {
			references = combinedReferences
			excludedConflictIDs = excludedConflictIDsCopy
		}
	}

	if len(references[StrongParentType]) == 0 {
		return nil, errors.Errorf("none of the provided strong parents can be referenced. Strong parents provided: %+v.", strongParents)
	}

	return references, nil
}

func (r *ReferenceProvider) ReferencesToMissingConflicts(issuingTime time.Time, amount int) (blockIDs BlockIDs) {
	blockIDs = NewBlockIDs()
	if amount == 0 {
		return blockIDs
	}

	for it := r.tangle.TipManager.tipsConflictTracker.MissingConflicts(amount).Iterator(); it.HasNext(); {
		blockID, blockIDErr := r.firstValidAttachment(it.Next(), issuingTime)
		if blockIDErr != nil {
			continue
		}

		blockIDs.Add(blockID)
	}

	return blockIDs
}

// addedReferenceForBlock returns the reference that is necessary to correct our opinion on the given block.
func (r *ReferenceProvider) addedReferencesForBlock(blkID BlockID, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (addedReferences ParentBlockIDs, success bool) {
	blkConflictIDs, err := r.tangle.Booker.BlockConflictIDs(blkID)
	if err != nil {
		r.tangle.OrphanageManager.OrphanBlock(blkID, errors.Errorf("conflictID of %s can't be retrieved: %w", blkID, err))
		return nil, false
	}

	addedReferences = NewParentBlockIDs()
	if blkConflictIDs.IsEmpty() {
		return addedReferences, true
	}

	if addedReferences, err = r.addedReferencesForConflicts(blkConflictIDs, issuingTime, excludedConflictIDs); err != nil {
		r.tangle.OrphanageManager.OrphanBlock(blkID, errors.Errorf("cannot pick up %s as strong parent: %w", blkID, err))
		return nil, false
	}

	// fmt.Println("excludedConflictIDs", excludedConflictIDs)
	// fmt.Println("addedReferencesForBlock", blkID, addedReferences)
	// A block might introduce too many references and cannot be picked up as a strong parent.
	if _, success := r.tryExtendReferences(NewParentBlockIDs(), addedReferences); !success {
		r.tangle.OrphanageManager.OrphanBlock(blkID, errors.Errorf("cannot pick up %s as strong parent: %w", blkID, err))
		return nil, false
	}

	return addedReferences, true
}

// addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (referencesToAdd ParentBlockIDs, err error) {
	referencesToAdd = NewParentBlockIDs()
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		// If we already expressed a dislike of the conflict (through another liked instead) we don't need to revisit this conflictID.
		if excludedConflictIDs.Has(conflictID) {
			continue
		}

		if adjust, referencedBlk, referenceErr := r.adjustOpinion(conflictID, issuingTime, excludedConflictIDs); referenceErr != nil {
			return nil, errors.Errorf("failed to create reference for %s: %w", conflictID, referenceErr)
		} else if adjust {
			referencesToAdd.Add(ShallowLikeParentType, referencedBlk)
		}
	}

	return referencesToAdd, nil
}

// adjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (adjust bool, blkID BlockID, err error) {
	for w := walker.New[utxo.TransactionID](false).Push(conflictID); w.HasNext(); {
		currentConflictID := w.Next()

		if likedConflictID, dislikedConflictIDs := r.tangle.OTVConsensusManager.LikedConflictMember(currentConflictID); !likedConflictID.IsEmpty() {
			// only triggers in first iteration
			if likedConflictID == conflictID {
				return false, EmptyBlockID, nil
			}

			if blkID, err = r.firstValidAttachment(likedConflictID, issuingTime); err != nil {
				continue
			}

			excludedConflictIDs.AddAll(r.tangle.Ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))

			return true, blkID, nil
		}

		// only walk deeper if we don't like "something else"
		r.tangle.Ledger.ConflictDAG.Storage.CachedConflict(currentConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			w.PushFront(conflict.Parents().Slice()...)
		})
	}

	return false, EmptyBlockID, errors.Newf("failed to create dislike for %s", conflictID)
}

// firstValidAttachment returns the first valid attachment of the given transaction.
func (r *ReferenceProvider) firstValidAttachment(txID utxo.TransactionID, issuingTime time.Time) (blkID BlockID, err error) {
	attachmentTime, blkID, err := r.tangle.Utils.FirstAttachment(txID)
	if err != nil {
		return EmptyBlockID, errors.Errorf("failed to find first attachment of Transaction with %s: %w", txID, err)
	}

	// TODO: we don't want to vote on anything that is in a committed epoch.
	if !r.validTime(attachmentTime, issuingTime) {
		return EmptyBlockID, errors.Errorf("attachment of %s with %s is too far in the past", txID, blkID)
	}

	return blkID, nil
}

func (r *ReferenceProvider) validTime(parentTime, issuingTime time.Time) bool {
	// TODO: we don't want to vote on anything that is in a committed epoch.
	return issuingTime.Sub(parentTime) < maxParentsTimeDifference
}

func (r *ReferenceProvider) validTimeForStrongParent(blkID BlockID, issuingTime time.Time) (valid bool) {
	r.tangle.Storage.Block(blkID).Consume(func(blk *Block) {
		valid = r.validTime(blk.IssuingTime(), issuingTime)
	})
	return valid
}

// checkPayloadLiked checks if the payload of a Block is liked.
func (r *ReferenceProvider) checkPayloadLiked(blkID BlockID) (err error) {
	conflictIDs, err := r.tangle.Booker.PayloadConflictIDs(blkID)
	if err != nil {
		return errors.Errorf("failed to determine payload conflictIDs of %s: %w", blkID, err)
	}

	if !r.tangle.Utils.AllConflictsLiked(conflictIDs) {
		return errors.Errorf("payload of %s is not liked: %s", blkID, conflictIDs)
	}

	return nil
}

// tryExtendReferences tries to extend the references with the given referencesToAdd.
func (r *ReferenceProvider) tryExtendReferences(references ParentBlockIDs, referencesToAdd ParentBlockIDs) (extendedReferences ParentBlockIDs, success bool) {
	if referencesToAdd.IsEmpty() {
		return references, true
	}

	extendedReferences = references.Clone()
	for referenceType, referencedBlockIDs := range referencesToAdd {
		extendedReferences.AddAll(referenceType, referencedBlockIDs)

		if len(extendedReferences[referenceType]) > MaxParentsCount {
			return nil, false
		}
	}

	return extendedReferences, true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
