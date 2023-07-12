package blockfactory

import (
	"time"

	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
)

// region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////

// ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
type ReferenceProvider struct {
	protocol *protocol.Protocol

	latestSlotIndexCallback        func() slot.Index
	timeSinceConfirmationThreshold time.Duration
}

// NewReferenceProvider creates a new ReferenceProvider instance.
func NewReferenceProvider(protocol *protocol.Protocol, timeSinceConfirmationThreshold time.Duration, latestSlotIndexCallback func() slot.Index) (newInstance *ReferenceProvider) {
	return &ReferenceProvider{
		protocol:                       protocol,
		latestSlotIndexCallback:        latestSlotIndexCallback,
		timeSinceConfirmationThreshold: timeSinceConfirmationThreshold,
	}
}

// References is an implementation of ReferencesFunc.
func (r *ReferenceProvider) References(payload payload.Payload, strongParents models.BlockIDs) (references models.ParentBlockIDs, err error) {
	references = models.NewParentBlockIDs()

	excludedConflictIDs := utxo.NewTransactionIDs()

	err = r.protocol.Engine().Ledger.MemPool().ConflictDAG().ReadConsistent(func(conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) error {
		for strongParent := range strongParents {
			excludedConflictIDsCopy := excludedConflictIDs.Clone()
			referencesToAdd, validStrongParent := r.addedReferencesForBlock(strongParent, excludedConflictIDsCopy, conflictDAG)
			if !validStrongParent {
				if !r.payloadLiked(strongParent, conflictDAG) {
					continue
				}

				referencesToAdd = models.NewParentBlockIDs().Add(models.WeakParentType, strongParent)
			} else {
				referencesToAdd.AddStrong(strongParent)
			}

			if combinedReferences, success := r.tryExtendReferences(references, referencesToAdd); success {
				references = combinedReferences
				excludedConflictIDs = excludedConflictIDsCopy
			}
		}

		if len(references[models.StrongParentType]) == 0 {
			return errors.Errorf("none of the provided strong parents can be referenced. Strong parents provided: %+v.", strongParents)
		}

		// This should be liked anyway, or at least it should be corrected by shallow like if we spend.
		// If a node spends something it doesn't like, then the payload is invalid as well.
		weakReferences, likeInsteadReferences, err := r.referencesFromUnacceptedInputs(payload, excludedConflictIDs, conflictDAG)
		if err != nil {
			return errors.Wrapf(err, "failed to create references for unnaccepted inputs")
		}

		references.AddAll(models.WeakParentType, weakReferences)
		references.AddAll(models.ShallowLikeParentType, likeInsteadReferences)

		// Include censored, pending conflicts if there are free weak parent spots.
		references.AddAll(models.WeakParentType, r.referencesToMissingConflicts(models.MaxParentsCount-len(references[models.WeakParentType]), conflictDAG))

		// Make sure that there's no duplicate between strong and weak parents.
		references.CleanupReferences()

		return nil
	})

	return references, err
}

func (r *ReferenceProvider) referencesToMissingConflicts(amount int, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (blockIDs models.BlockIDs) {
	blockIDs = models.NewBlockIDs()
	if amount == 0 {
		return blockIDs
	}

	for it := r.protocol.TipManager.TipsConflictTracker.MissingConflicts(amount, conflictDAG).Iterator(); it.HasNext(); {
		conflictID := it.Next()

		// TODO: make sure that timestamp monotonicity is not broken
		// TODO: need to check of every parent instead of adjusting the block's issuing time
		attachment, err := r.latestValidAttachment(conflictID)
		if attachment == nil || err != nil {
			// panic("first attachment should be valid")
			continue
		}

		// TODO: use earliest instead? to avoid commitment monotonicity issue
		// attachment := r.protocol.Engine().Tangle.GetLatestAttachment(it.Next())
		// if attachment == nil {
		// 	panic("attachment should not be nil")
		// }

		blockIDs.Add(attachment.ID())
	}

	return blockIDs
}

func (r *ReferenceProvider) referencesFromUnacceptedInputs(payload payload.Payload, excludedConflictIDs utxo.TransactionIDs, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (weakParents models.BlockIDs, likeInsteadParents models.BlockIDs, err error) {
	weakParents = models.NewBlockIDs()
	likeInsteadParents = models.NewBlockIDs()

	engineInstance := r.protocol.Engine()
	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
	tx, isTx := payload.(utxo.Transaction)
	if !isTx {
		return weakParents, likeInsteadParents, nil
	}

	referencedTransactions := engineInstance.Ledger.MemPool().Utils().ReferencedTransactions(tx)
	for it := referencedTransactions.Iterator(); it.HasNext(); {
		referencedTransactionID := it.Next()

		if len(weakParents) == models.MaxParentsCount {
			return weakParents, likeInsteadParents, nil
		}

		if !engineInstance.Ledger.MemPool().Utils().TransactionConfirmationState(referencedTransactionID).IsAccepted() {
			latestAttachment := engineInstance.Tangle.Booker().GetLatestAttachment(referencedTransactionID)
			if latestAttachment == nil {
				continue
			}

			// do not add a block from an already committed slot as weak parent
			if latestAttachment.ID().Index() <= r.latestSlotIndexCallback() {
				continue
			}

			transactionConflictIDs := engineInstance.Tangle.Booker().TransactionConflictIDs(latestAttachment)
			if transactionConflictIDs.IsEmpty() {
				weakParents.Add(latestAttachment.ID())
				continue
			}

			for conflictIterator := transactionConflictIDs.Iterator(); conflictIterator.HasNext(); {
				transactionConflictID := conflictIterator.Next()
				if excludedConflictIDs.Has(transactionConflictID) {
					continue
				}

				if adjust, referencedBlk, referenceErr := r.adjustOpinion(transactionConflictID, excludedConflictIDs, conflictDAG); referenceErr != nil {
					return nil, nil, errors.Wrapf(referenceErr, "failed to correct opinion for weak parent with unaccepted output %s", referencedTransactionID)
				} else if adjust {
					if referencedBlk != models.EmptyBlockID {
						likeInsteadParents.Add(referencedBlk)
					} else {
						return nil, nil, errors.Errorf("failed to correct opinion for weak parent with unaccepted output %s", referencedTransactionID)
					}
				}
			}

			weakParents.Add(latestAttachment.ID())
		}
	}

	return weakParents, likeInsteadParents, nil
}

// addedReferenceForBlock returns the reference that is necessary to correct our opinion on the given block.
func (r *ReferenceProvider) addedReferencesForBlock(blockID models.BlockID, excludedConflictIDs utxo.TransactionIDs, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (addedReferences models.ParentBlockIDs, success bool) {
	engineInstance := r.protocol.Engine()

	block, exists := engineInstance.Tangle.Booker().Block(blockID)
	if !exists {
		return nil, false
	}
	blockConflicts := engineInstance.Tangle.Booker().BlockConflicts(block)

	addedReferences = models.NewParentBlockIDs()
	if blockConflicts.IsEmpty() {
		return addedReferences, true
	}

	var err error
	if addedReferences, err = r.addedReferencesForConflicts(blockConflicts, excludedConflictIDs, conflictDAG); err != nil {
		// Delete the tip if we could not pick it up.
		if schedulerBlock, schedulerBlockExists := r.protocol.CongestionControl.Scheduler().Block(blockID); schedulerBlockExists {
			r.protocol.TipManager.InvalidateTip(schedulerBlock)
		}
		return nil, false
	}

	//// We could not refer to any block to fix the opinion, so we add the tips' strong parents to the tip pool.
	//if addedReferences == nil {
	//	if block, exists := r.protocol.Engine().Tangle.Booker().Block(blockID); exists {
	//		block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
	//			if schedulerBlock, schedulerBlockExists := r.protocol.CongestionControl.Scheduler().Block(parentBlockID); schedulerBlockExists {
	//				r.protocol.TipManager.AddTipNonMonotonic(schedulerBlock)
	//			}
	//			return true
	//		})
	//	}
	//	fmt.Println(">> could not fix opinion", blockID)
	//	return nil, false
	//}

	// A block might introduce too many references and cannot be picked up as a strong parent.
	if _, success = r.tryExtendReferences(models.NewParentBlockIDs(), addedReferences); !success {
		return nil, false
	}

	return addedReferences, true
}

// addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, excludedConflictIDs utxo.TransactionIDs, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (referencesToAdd models.ParentBlockIDs, err error) {
	referencesToAdd = models.NewParentBlockIDs()

	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		// If we already expressed a dislike of the conflict (through another liked instead) we don't need to revisit this conflictID.
		if excludedConflictIDs.Has(conflictID) {
			continue
		}

		adjust, referencedBlk, referenceErr := r.adjustOpinion(conflictID, excludedConflictIDs, conflictDAG)
		if referenceErr != nil {
			return nil, errors.Wrapf(referenceErr, "failed to create reference for %s", conflictID)
		}

		if adjust {
			referencesToAdd.Add(models.ShallowLikeParentType, referencedBlk)
		}
	}

	return referencesToAdd, nil
}

// adjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, excludedConflictIDs utxo.TransactionIDs, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (adjust bool, attachmentID models.BlockID, err error) {
	engineInstance := r.protocol.Engine()

	likedConflictID := conflictDAG.LikedInstead(advancedset.New(conflictID))
	// if likedConflictID is empty, then conflictID is liked and doesn't need to be corrected
	if likedConflictID.IsEmpty() {
		return false, models.EmptyBlockID, nil
	}

	if err = likedConflictID.ForEach(func(likedConflictID utxo.TransactionID) (err error) {
		attachment, err := r.latestValidAttachment(likedConflictID)
		// TODO: make sure that timestamp monotonicity is held
		if err != nil {
			return err
		}

		attachmentID = attachment.ID()

		excludedConflictIDs.AddAll(engineInstance.Ledger.MemPool().Utils().ConflictIDsInFutureCone(lo.Return1(conflictDAG.ConflictingConflicts(likedConflictID))))

		return nil
	}); err != nil {
		return false, models.EmptyBlockID, err
	}

	if attachmentID == models.EmptyBlockID {
		return false, attachmentID, xerrors.Errorf("could not find attachment to fix conflict %s", conflictID)
	}
	return true, attachmentID, nil
}

// latestValidAttachment returns the first valid attachment of the given transaction.
func (r *ReferenceProvider) latestValidAttachment(txID utxo.TransactionID) (block *booker.Block, err error) {
	block = r.protocol.Engine().Tangle.Booker().GetLatestAttachment(txID)
	if block == nil {
		return nil, errors.Errorf("could not obtain latest attachment for %s", txID)
	}

	if acceptedTime := r.protocol.Engine().Clock.Accepted().Time(); block.IssuingTime().Before(acceptedTime.Add(-r.timeSinceConfirmationThreshold)) {
		return nil, errors.Errorf("attachment of %s with %s is too far in the past relative to AcceptedTime %s", txID, block.ID(), acceptedTime.String())
	}

	if committableSlot := r.latestSlotIndexCallback(); block.ID().Index() <= committableSlot {
		return nil, errors.Errorf("attachment of %s with %s is too far in the past as current committable slot is %d", txID, block.ID(), committableSlot)
	}

	return block, nil
}

// payloadLiked checks if the payload of a Block is liked.
func (r *ReferenceProvider) payloadLiked(blockID models.BlockID, conflictDAG conflictdag.ReadLockedConflictDAG[utxo.TransactionID, utxo.OutputID, models.BlockVotePower]) (liked bool) {
	engineInstance := r.protocol.Engine()

	block, exists := engineInstance.Tangle.Booker().Block(blockID)
	if !exists {
		return false
	}

	for conflicts := engineInstance.Tangle.Booker().TransactionConflictIDs(block).Iterator(); conflicts.HasNext(); {
		if !conflictDAG.LikedInstead(advancedset.New(conflicts.Next())).IsEmpty() {
			return false
		}
	}

	return true
}

// tryExtendReferences tries to extend the references with the given referencesToAdd.
func (r *ReferenceProvider) tryExtendReferences(references models.ParentBlockIDs, referencesToAdd models.ParentBlockIDs) (extendedReferences models.ParentBlockIDs, success bool) {
	if referencesToAdd.IsEmpty() {
		return references, true
	}

	extendedReferences = references.Clone()
	for referenceType, referencedBlockIDs := range referencesToAdd {
		extendedReferences.AddAll(referenceType, referencedBlockIDs)

		if len(extendedReferences[referenceType]) > models.MaxParentsCount {
			return nil, false
		}
	}

	return extendedReferences, true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
