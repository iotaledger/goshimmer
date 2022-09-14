package blockfactory

//
// import (
// 	"github.com/cockroachdb/errors"
// 	"github.com/iotaledger/hive.go/core/generics/walker"
//
// 	"github.com/iotaledger/goshimmer/packages/core/epoch"
// 	"github.com/iotaledger/goshimmer/packages/protocol/engine"
// 	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
// 	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models/payload"
// 	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
// 	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
// )
//
// // region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////
//
// // ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
// type ReferenceProvider struct {
// 	engine                   engine.Engine
// 	latestEpochIndexCallback func() epoch.Index
// }
//
// // NewReferenceProvider creates a new ReferenceProvider instance.
// func NewReferenceProvider() (newInstance *ReferenceProvider) {
// 	return &ReferenceProvider{}
// }
//
// // TODO: blocks vs. blockIDs
//
// // References is an implementation of ReferencesFunc.
// func (r *ReferenceProvider) References(payload payload.Payload, strongParents models.BlockIDs) (references models.ParentBlockIDs, err error) {
// 	references = models.NewParentBlockIDs()
//
// 	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
// 	// if tx, isTx := payload.(utxo.Transaction); isTx {
// 	// 	referencedTxs := r.ledger.Utils.ReferencedTransactions(tx)
// 	// 	for it := referencedTxs.Iterator(); it.HasNext(); {
// 	// 		referencedTx := it.Next()
// 	// 		if !r.confirmationOracle.IsTransactionConfirmed(referencedTx) {
// 	// 			latestAttachment := r.blockFactory.LatestAttachment(referencedTx)
// 	// 			if latestAttachment == nil {
// 	// 				continue
// 	// 			}
// 	// 			timeDifference := clock.SyncedTime().Sub(latestAttachment.IssuingTime())
// 	// 			// If the latest attachment of the transaction we are consuming is too old we are not
// 	// 			// able to add it is a weak parent.
// 	// 			if timeDifference <= maxParentsTimeDifference {
// 	// 				if len(references[models.WeakParentType]) == models.MaxParentsCount {
// 	// 					return references, nil
// 	// 				}
// 	// 				references.Add(models.WeakParentType, latestAttachment.ID())
// 	// 			}
// 	// 		}
// 	// 	}
// 	// }
//
// 	excludedConflictIDs := utxo.NewTransactionIDs()
//
// 	for strongParent := range strongParents {
// 		excludedConflictIDsCopy := excludedConflictIDs.Clone()
// 		referencesToAdd, validStrongParent := r.addedReferencesForBlock(strongParent, excludedConflictIDsCopy)
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
// // addedReferenceForBlock returns the reference that is necessary to correct our opinion on the given block.
// func (r *ReferenceProvider) addedReferencesForBlock(blockID models.BlockID, excludedConflictIDs utxo.TransactionIDs) (addedReferences models.ParentBlockIDs, success bool) {
// 	blockConflicts := r.engine.Tangle.Booker.BlockConflicts(blockID)
//
// 	addedReferences = models.NewParentBlockIDs()
// 	if blockConflicts.IsEmpty() {
// 		return addedReferences, true
// 	}
//
// 	var err error
// 	if addedReferences, err = r.addedReferencesForConflicts(blockConflicts, excludedConflictIDs); err != nil {
// 		// r.engine.Tangle.BlockDAG.SetOrphaned()
// 		// TODO: .OrphanBlock(blockID, errors.Errorf("cannot pick up %s as strong parent: %w", blockID, err))
// 		return nil, false
// 	}
//
// 	// A block might introduce too many references and cannot be picked up as a strong parent.
// 	if _, success := r.tryExtendReferences(models.NewParentBlockIDs(), addedReferences); !success {
// 		return nil, false
// 	}
//
// 	return addedReferences, true
// }
//
// // addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
// func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, excludedConflictIDs utxo.TransactionIDs) (referencesToAdd models.ParentBlockIDs, err error) {
// 	referencesToAdd = models.NewParentBlockIDs()
// 	for it := conflictIDs.Iterator(); it.HasNext(); {
// 		conflictID := it.Next()
//
// 		// If we already expressed a dislike of the conflict (through another liked instead) we don't need to revisit this conflictID.
// 		if excludedConflictIDs.Has(conflictID) {
// 			continue
// 		}
//
// 		if adjust, referencedBlk, referenceErr := r.adjustOpinion(conflictID, excludedConflictIDs); referenceErr != nil {
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
// func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, excludedConflictIDs utxo.TransactionIDs) (adjust bool, blkID models.BlockID, err error) {
// 	for w := walker.New[utxo.TransactionID](false).Push(conflictID); w.HasNext(); {
// 		currentConflictID := w.Next()
//
// 		if likedConflictID, dislikedConflictIDs := r.engine.Consensus.LikedConflictMember(currentConflictID); !likedConflictID.IsEmpty() {
// 			// only triggers in first iteration
// 			if likedConflictID == conflictID {
// 				return false, models.EmptyBlockID, nil
// 			}
//
// 			if blkID, err = r.firstValidAttachment(likedConflictID); err != nil {
// 				continue
// 			}
//
// 			excludedConflictIDs.AddAll(r.engine.Ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))
//
// 			return true, blkID, nil
// 		}
//
// 		// only walk deeper if we don't like "something else"
// 		r.engine.Ledger.ConflictDAG.Storage.CachedConflict(currentConflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
// 			w.PushFront(conflict.Parents().Slice()...)
// 		})
// 	}
//
// 	return false, models.EmptyBlockID, errors.Newf("failed to create dislike for %s", conflictID)
// }
//
// // firstValidAttachment returns the first valid attachment of the given transaction.
// func (r *ReferenceProvider) firstValidAttachment(txID utxo.TransactionID) (blockID models.BlockID, err error) {
// 	block := r.engine.Tangle.Booker.GetEarliestAttachment(txID)
//
// 	committableEpoch := r.latestEpochIndexCallback()
// 	if block.ID().Index() <= committableEpoch {
// 		return models.EmptyBlockID, errors.Errorf("attachment of %s with %s is too far in the past as current committable epoch is %d", txID, block.ID(), committableEpoch)
// 	}
//
// 	return block.ID(), nil
// }
//
// // checkPayloadLiked checks if the payload of a Block is liked.
// func (r *ReferenceProvider) checkPayloadLiked(blockID models.BlockID) (liked bool) {
// 	conflictIDs := r.engine.Tangle.Booker.PayloadConflictIDs(blockID)
//
// 	for it := conflictIDs.Iterator(); it.HasNext(); {
// 		if !r.engine.Consensus.ConflictLiked(it.Next()) {
// 			return false
// 		}
// 	}
//
// 	return true
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
