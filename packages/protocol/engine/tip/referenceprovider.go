package tip

// import (
// 	"time"
//
// 	"github.com/cockroachdb/errors"
//
// 	"github.com/iotaledger/goshimmer/packages/core/clock"
// 	"github.com/iotaledger/goshimmer/packages/core/tangleold"
// 	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
// 	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
// 	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
// 	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
// 	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
// )
//
// // region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////
//
// // ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
// type ReferenceProvider struct {
// 	tipManager          *tangleold.Manager
// 	ledger              *ledger.Ledger
// 	booker              *tangleold.Booker
// 	orphanageManager    *tangleold.OrphanageManager
// 	utils               *tangleold.Utils
// 	otvConsensusManager *tangleold.OTVConsensusManager
//
// 	confirmationOracle tangleold.ConfirmationOracle
// 	blockFactory       *BlockFactory
// }
//
// // NewReferenceProvider creates a new ReferenceProvider instance.
// func NewReferenceProvider() (newInstance *ReferenceProvider) {
// 	return &ReferenceProvider{}
// }
//
// // References is an implementation of ReferencesFunc.
// func (r *ReferenceProvider) References(payload payload.Payload, strongParents models.BlockIDs, issuingTime time.Time) (references models.ParentBlockIDs, err error) {
// 	references = models.NewParentBlockIDs()
//
// 	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
// 	if tx, isTx := payload.(utxo.Transaction); isTx {
// 		referencedTxs := r.ledger.Utils.ReferencedTransactions(tx)
// 		for it := referencedTxs.Iterator(); it.HasNext(); {
// 			referencedTx := it.Next()
// 			if !r.confirmationOracle.IsTransactionConfirmed(referencedTx) {
// 				latestAttachment := r.blockFactory.LatestAttachment(referencedTx)
// 				if latestAttachment == nil {
// 					continue
// 				}
// 				timeDifference := clock.SyncedTime().Sub(latestAttachment.IssuingTime())
// 				// If the latest attachment of the transaction we are consuming is too old we are not
// 				// able to add it is a weak parent.
// 				if timeDifference <= maxParentsTimeDifference {
// 					if len(references[models.WeakParentType]) == models.MaxParentsCount {
// 						return references, nil
// 					}
// 					references.Add(models.WeakParentType, latestAttachment.ID())
// 				}
// 			}
// 		}
// 	}
//
// 	excludedConflictIDs := utxo.NewTransactionIDs()
//
// 	for strongParent := range strongParents {
// 		excludedConflictIDsCopy := excludedConflictIDs.Clone()
// 		referencesToAdd, validStrongParent := r.addedReferencesForBlock(strongParent, issuingTime, excludedConflictIDsCopy)
// 		if !validStrongParent {
// 			if err = r.checkPayloadLiked(strongParent); err != nil {
// 				continue
// 			}
//
// 			referencesToAdd = models.NewParentBlockIDs().Add(models.WeakParentType, strongParent)
// 		} else {
// 			referencesToAdd.AddStrong(strongParent)
// 		}
//
// 		if combinedReferences, success := r.tryExtendReferences(references, referencesToAdd); success {
// 			references = combinedReferences
// 			excludedConflictIDs = excludedConflictIDsCopy
// 		}
// 	}
//
// 	if len(references[models.StrongParentType]) == 0 {
// 		return nil, errors.Errorf("none of the provided strong parents can be referenced. Strong parents provided: %+v.", strongParents)
// 	}
//
// 	return references, nil
// }
//
// func (r *ReferenceProvider) ReferencesToMissingConflicts(issuingTime time.Time, amount int) (blockIDs models.BlockIDs) {
// 	blockIDs = models.NewBlockIDs()
// 	if amount == 0 {
// 		return blockIDs
// 	}
//
// 	for it := r.tipManager.tipsConflictTracker.MissingConflicts(amount).Iterator(); it.HasNext(); {
// 		blockID, blockIDErr := r.firstValidAttachment(it.Next(), issuingTime)
// 		if blockIDErr != nil {
// 			continue
// 		}
//
// 		blockIDs.Add(blockID)
// 	}
//
// 	return blockIDs
// }
//
// // addedReferenceForBlock returns the reference that is necessary to correct our opinion on the given block.
// func (r *ReferenceProvider) addedReferencesForBlock(blkID models.BlockID, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (addedReferences models.ParentBlockIDs, success bool) {
// 	blkConflictIDs, err := r.booker.BlockConflictIDs(blkID)
// 	if err != nil {
// 		r.orphanageManager.OrphanBlock(blkID, errors.Errorf("conflictID of %s can't be retrieved: %w", blkID, err))
// 		return nil, false
// 	}
//
// 	addedReferences = models.NewParentBlockIDs()
// 	if blkConflictIDs.IsEmpty() {
// 		return addedReferences, true
// 	}
//
// 	if addedReferences, err = r.addedReferencesForConflicts(blkConflictIDs, issuingTime, excludedConflictIDs); err != nil {
// 		r.orphanageManager.OrphanBlock(blkID, errors.Errorf("cannot pick up %s as strong parent: %w", blkID, err))
// 		return nil, false
// 	}
//
// 	// fmt.Println("excludedConflictIDs", excludedConflictIDs)
// 	// fmt.Println("addedReferencesForBlock", blkID, addedReferences)
// 	// A block might introduce too many references and cannot be picked up as a strong parent.
// 	if _, success := r.tryExtendReferences(models.NewParentBlockIDs(), addedReferences); !success {
// 		r.orphanageManager.OrphanBlock(blkID, errors.Errorf("cannot pick up %s as strong parent: %w", blkID, err))
// 		return nil, false
// 	}
//
// 	return addedReferences, true
// }
//
// // addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
// func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (referencesToAdd models.ParentBlockIDs, err error) {
// 	referencesToAdd = models.NewParentBlockIDs()
// 	for it := conflictIDs.Iterator(); it.HasNext(); {
// 		conflictID := it.Next()
//
// 		// If we already expressed a dislike of the conflict (through another liked instead) we don't need to revisit this conflictID.
// 		if excludedConflictIDs.Has(conflictID) {
// 			continue
// 		}
//
// 		if adjust, referencedBlk, referenceErr := r.adjustOpinion(conflictID, issuingTime, excludedConflictIDs); referenceErr != nil {
// 			return nil, errors.Errorf("failed to create reference for %s: %w", conflictID, referenceErr)
// 		} else if adjust {
// 			referencesToAdd.Add(models.ShallowLikeParentType, referencedBlk)
// 		}
// 	}
//
// 	return referencesToAdd, nil
// }
//
// // adjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
// func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, issuingTime time.Time, excludedConflictIDs utxo.TransactionIDs) (adjust bool, blkID models.BlockID, err error) {
// 	for w := walker.New[utxo.TransactionID](false).Push(conflictID); w.HasNext(); {
// 		currentConflictID := w.Next()
//
// 		if likedConflictID, dislikedConflictIDs := r.otvConsensusManager.LikedConflictMember(currentConflictID); !likedConflictID.IsEmpty() {
// 			// only triggers in first iteration
// 			if likedConflictID == conflictID {
// 				return false, models.EmptyBlockID, nil
// 			}
//
// 			if blkID, err = r.firstValidAttachment(likedConflictID, issuingTime); err != nil {
// 				continue
// 			}
//
// 			excludedConflictIDs.AddAll(r.ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))
//
// 			return true, blkID, nil
// 		}
//
// 		// only walk deeper if we don't like "something else"
// 		r.ledger.ConflictDAG.Storage.CachedConflict(currentConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
// 			w.PushFront(conflict.Parents().Slice()...)
// 		})
// 	}
//
// 	return false, models.EmptyBlockID, errors.Newf("failed to create dislike for %s", conflictID)
// }
//
// // firstValidAttachment returns the first valid attachment of the given transaction.
// func (r *ReferenceProvider) firstValidAttachment(txID utxo.TransactionID, issuingTime time.Time) (blkID models.BlockID, err error) {
// 	attachmentTime, blkID, err := r.utils.FirstAttachment(txID)
// 	if err != nil {
// 		return models.EmptyBlockID, errors.Errorf("failed to find first attachment of Transaction with %s: %w", txID, err)
// 	}
//
// 	// TODO: we don't want to vote on anything that is in a committed epoch.
// 	if !r.validTime(attachmentTime, issuingTime) {
// 		return models.EmptyBlockID, errors.Errorf("attachment of %s with %s is too far in the past", txID, blkID)
// 	}
//
// 	return blkID, nil
// }
//
// func (r *ReferenceProvider) validTime(parentTime, issuingTime time.Time) bool {
// 	// TODO: we don't want to vote on anything that is in a committed epoch.
// 	return issuingTime.Sub(parentTime) < maxParentsTimeDifference
// }
//
// func (r *ReferenceProvider) validTimeForStrongParent(blkID models.BlockID, issuingTime time.Time) (valid bool) {
// 	r.tangle.Storage.Block(blkID).Consume(func(blk *models.Block) {
// 		valid = r.validTime(blk.IssuingTime(), issuingTime)
// 	})
// 	return valid
// }
//
// // checkPayloadLiked checks if the payload of a Block is liked.
// func (r *ReferenceProvider) checkPayloadLiked(blkID models.BlockID) (err error) {
// 	conflictIDs, err := r.booker.PayloadConflictIDs(blkID)
// 	if err != nil {
// 		return errors.Errorf("failed to determine payload conflictIDs of %s: %w", blkID, err)
// 	}
//
// 	if !r.utils.AllConflictsLiked(conflictIDs) {
// 		return errors.Errorf("payload of %s is not liked: %s", blkID, conflictIDs)
// 	}
//
// 	return nil
// }
//
// // tryExtendReferences tries to extend the references with the given referencesToAdd.
// func (r *ReferenceProvider) tryExtendReferences(references models.ParentBlockIDs, referencesToAdd models.ParentBlockIDs) (extendedReferences models.ParentBlockIDs, success bool) {
// 	if referencesToAdd.IsEmpty() {
// 		return references, true
// 	}
//
// 	extendedReferences = references.Clone()
// 	for referenceType, referencedBlockIDs := range referencesToAdd {
// 		extendedReferences.AddAll(referenceType, referencedBlockIDs)
//
// 		if len(extendedReferences[referenceType]) > models.MaxParentsCount {
// 			return nil, false
// 		}
// 	}
//
// 	return extendedReferences, true
// }
//
// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
