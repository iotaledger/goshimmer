package blockfactory

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

// region ReferenceProvider ////////////////////////////////////////////////////////////////////////////////////////////

// ReferenceProvider is a component that takes care of creating the correct references when selecting tips.
type ReferenceProvider struct {
	protocol *protocol.Protocol

	latestEpochIndexCallback       func() epoch.Index
	timeSinceConfirmationThreshold time.Duration
}

// NewReferenceProvider creates a new ReferenceProvider instance.
func NewReferenceProvider(protocol *protocol.Protocol, timeSinceConfirmationThreshold time.Duration, latestEpochIndexCallback func() epoch.Index) (newInstance *ReferenceProvider) {
	return &ReferenceProvider{
		protocol:                       protocol,
		latestEpochIndexCallback:       latestEpochIndexCallback,
		timeSinceConfirmationThreshold: timeSinceConfirmationThreshold,
	}
}

// References is an implementation of ReferencesFunc.
func (r *ReferenceProvider) References(payload payload.Payload, strongParents models.BlockIDs) (references models.ParentBlockIDs, err error) {
	references = models.NewParentBlockIDs()

	references[models.WeakParentType], err = r.weakParentsFromUnacceptedInputs(payload)
	if err != nil {
		return
	}

	excludedConflictIDs := utxo.NewTransactionIDs()

	for strongParent := range strongParents {
		excludedConflictIDsCopy := excludedConflictIDs.Clone()
		referencesToAdd, validStrongParent := r.addedReferencesForBlock(strongParent, excludedConflictIDsCopy)
		if !validStrongParent {
			if !r.payloadLiked(strongParent) {
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
		return nil, errors.Errorf("none of the provided strong parents can be referenced. Strong parents provided: %+v.", strongParents)
	}

	// Uncensor pending conflicts
	references[models.WeakParentType].AddAll(r.referencesToMissingConflicts(models.MaxParentsCount - len(references[models.WeakParentType])))

	// Make sure that there's no duplicate between strong and weak parents.
	references.RemoveDuplicatesFromWeak()

	return references, nil
}

func (r *ReferenceProvider) referencesToMissingConflicts(amount int) (blockIDs models.BlockIDs) {
	blockIDs = models.NewBlockIDs()
	if amount == 0 {
		return blockIDs
	}

	for it := r.protocol.TipManager.TipsConflictTracker.MissingConflicts(amount).Iterator(); it.HasNext(); {
		conflictID := it.Next()

		// TODO: make sure that timestamp monotonicity is not broken
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

		// Check commitment monotonicity for the attachment.
		if attachment.Commitment().Index() > r.protocol.Engine().Storage.Settings.LatestCommitment().Index() {
			continue
		}

		blockIDs.Add(attachment.ID())
	}

	return blockIDs
}

func (r *ReferenceProvider) weakParentsFromUnacceptedInputs(payload payload.Payload) (weakParents models.BlockIDs, err error) {
	weakParents = models.NewBlockIDs()
	engineInstance := r.protocol.Engine()
	// If the payload is a transaction we will weakly reference unconfirmed transactions it is consuming.
	tx, isTx := payload.(utxo.Transaction)
	if !isTx {
		return weakParents, nil
	}

	referencedTransactions := engineInstance.Ledger.Utils.ReferencedTransactions(tx)
	for it := referencedTransactions.Iterator(); it.HasNext(); {
		referencedTransactionID := it.Next()

		if len(weakParents) == models.MaxParentsCount {
			return weakParents, nil
		}

		if !engineInstance.Ledger.Utils.TransactionConfirmationState(referencedTransactionID).IsAccepted() {
			latestAttachment := engineInstance.Tangle.Booker.GetLatestAttachment(referencedTransactionID)
			if latestAttachment == nil {
				continue
			}

			committableEpoch := r.latestEpochIndexCallback()

			if latestAttachment.ID().Index() <= committableEpoch {
				continue
			}

			weakParents.Add(latestAttachment.ID())
		}
	}

	return weakParents, nil
}

// addedReferenceForBlock returns the reference that is necessary to correct our opinion on the given block.
func (r *ReferenceProvider) addedReferencesForBlock(blockID models.BlockID, excludedConflictIDs utxo.TransactionIDs) (addedReferences models.ParentBlockIDs, success bool) {
	engineInstance := r.protocol.Engine()

	block, exists := engineInstance.Tangle.Booker.Block(blockID)
	if !exists {
		fmt.Println(">> block does not exist", blockID)
		return nil, false
	}
	blockConflicts := engineInstance.Tangle.Booker.BlockConflicts(block)

	addedReferences = models.NewParentBlockIDs()
	if blockConflicts.IsEmpty() {
		return addedReferences, true
	}

	var err error
	if addedReferences, err = r.addedReferencesForConflicts(blockConflicts, excludedConflictIDs); err != nil {
		// Delete the tip if we could not pick it up.
		fmt.Println(">> error adding references", blockID)
		if schedulerBlock, schedulerBlockExists := r.protocol.CongestionControl.Scheduler().Block(blockID); schedulerBlockExists {
			fmt.Println(">> tip removed", blockID)
			r.protocol.TipManager.DeleteTip(schedulerBlock)
		}
		return nil, false
	}

	// We could not refer to any block to fix the opinion, so we add the tips' strong parents to the tip pool.
	if addedReferences == nil {
		if block, exists := r.protocol.Engine().Tangle.Booker.Block(blockID); exists {
			block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
				if schedulerBlock, schedulerBlockExists := r.protocol.CongestionControl.Scheduler().Block(parentBlockID); schedulerBlockExists {
					r.protocol.TipManager.AddTipNonMonotonic(schedulerBlock)
				}
				return true
			})
		}
		fmt.Println(">> could not fix opinion", blockID)
		return nil, false
	}

	// A block might introduce too many references and cannot be picked up as a strong parent.
	if _, success = r.tryExtendReferences(models.NewParentBlockIDs(), addedReferences); !success {
		fmt.Println(">> tryExtendedReferences", blockID)
		return nil, false
	}

	return addedReferences, true
}

// addedReferencesForConflicts returns the references that are necessary to correct our opinion on the given conflicts.
func (r *ReferenceProvider) addedReferencesForConflicts(conflictIDs utxo.TransactionIDs, excludedConflictIDs utxo.TransactionIDs) (referencesToAdd models.ParentBlockIDs, err error) {
	referencesToAdd = models.NewParentBlockIDs()

	// If any of the conflict is rejected we cannot pick up the block as a parent, and we delete it from the tipset.
	if r.protocol.Engine().Tangle.Booker.Ledger.ConflictDAG.ConfirmationState(conflictIDs).IsRejected() {
		return nil, errors.Errorf("the given conflicts are rejected: %s", conflictIDs)
	}

	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		// If we already expressed a dislike of the conflict (through another liked instead) we don't need to revisit this conflictID.
		if excludedConflictIDs.Has(conflictID) {
			continue
		}

		if adjust, referencedBlk, referenceErr := r.adjustOpinion(conflictID, excludedConflictIDs); referenceErr != nil {
			return nil, errors.Wrapf(referenceErr, "failed to create reference for %s", conflictID)
		} else if adjust {
			if referencedBlk != models.EmptyBlockID {
				referencesToAdd.Add(models.ShallowLikeParentType, referencedBlk)
			} else {
				// We could not find a block that we could reference to fix this strong parent, but we don't want to delete the tip.
				return nil, nil
			}
		}
	}

	return referencesToAdd, nil
}

// adjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
func (r *ReferenceProvider) adjustOpinion(conflictID utxo.TransactionID, excludedConflictIDs utxo.TransactionIDs) (adjust bool, attachmentID models.BlockID, err error) {
	engineInstance := r.protocol.Engine()

	likedConflictID, dislikedConflictIDs := engineInstance.Consensus.ConflictResolver.AdjustOpinion(conflictID)

	if likedConflictID.IsEmpty() {
		return false, models.EmptyBlockID, errors.Errorf("failed to adjust opinion for %s", conflictID)
	}

	if likedConflictID == conflictID {
		return false, models.EmptyBlockID, nil
	}

	attachment, err := r.latestValidAttachment(likedConflictID)
	// TODO: make sure that timestamp monotonicity is held
	if err != nil {
		return false, models.EmptyBlockID, err
	}

	// Check if the attachment has a monotonic commitment.
	if attachment.Commitment().Index() > r.protocol.Engine().Storage.Settings.LatestCommitment().Index() {
		fmt.Printf(">> I want commitment block %s with commitment %d but I have %d\n", attachment.ID(), attachment.Commitment().Index(), r.protocol.Engine().Storage.Settings.LatestCommitment().Index())
		return true, models.EmptyBlockID, nil
	}

	excludedConflictIDs.AddAll(engineInstance.Ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))

	return true, attachment.ID(), nil
}

// latestValidAttachment returns the first valid attachment of the given transaction.
func (r *ReferenceProvider) latestValidAttachment(txID utxo.TransactionID) (block *booker.Block, err error) {
	block = r.protocol.Engine().Tangle.Booker.GetLatestAttachment(txID)
	if block == nil {
		return nil, errors.Errorf("could not obtain latest attachment for %s", txID)
	}

	if acceptedTime := r.protocol.Engine().Clock.AcceptedTime(); block.IssuingTime().Before(acceptedTime.Add(-r.timeSinceConfirmationThreshold)) {
		return nil, errors.Errorf("attachment of %s with %s is too far in the past relative to AcceptedTime %s", txID, block.ID(), acceptedTime.String())
	}

	// if block.IsSubjectivelyInvalid() {
	// 	fmt.Println(">> sub invalid", block.ID())
	// 	// The block returned will be corresponding to the next heaviest originalConflict in the set.
	// 	block = nil
	// 	originalConflict, exists := r.protocol.Engine().Ledger.ConflictDAG.Conflict(txID)
	// 	if exists {
	// 		r.protocol.Engine().Consensus.ForEachConnectedConflictingConflictInDescendingOrder(originalConflict, func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	// 			if block != nil {
	// 				return
	// 			}
	//
	// 			fmt.Println(">> trying for conflict", conflict.ID())
	//
	// 			if originalConflict.ID() == conflict.ID() {
	// 				fmt.Println(">> blip")
	// 				return
	// 			}
	//
	// 			bookerBlock := r.protocol.Engine().Tangle.Booker.GetEarliestAttachment(conflict.ID())
	// 			if bookerBlock == nil {
	// 				fmt.Println(">> blop")
	// 				return
	// 			}
	//
	// 			blockInner, exists := r.protocol.Engine().Tangle.VirtualVoting.Block(bookerBlock.ID())
	// 			if !exists {
	// 				fmt.Println(">> men men")
	// 				return
	// 			}
	//
	// 			if blockInner.IsSubjectivelyInvalid() {
	// 				fmt.Println(">> more sub invalid", blockInner.ID())
	// 				return
	// 			}
	//
	// 			fmt.Println(">> FOUND!!", blockInner.ID())
	// 			block = blockInner
	// 		})
	// 	}
	//
	// 	if block == nil {
	// 		return nil, errors.Errorf("no attachment for conflict members of %s are valid", txID)
	// 		// return nil, errors.Errorf("attachment of %s with %s is subjectively invalid", txID, bookerBlock.ID())
	// 	}
	// }

	if committableEpoch := r.latestEpochIndexCallback(); block.ID().Index() <= committableEpoch {
		return nil, errors.Errorf("attachment of %s with %s is too far in the past as current committable epoch is %d", txID, block.ID(), committableEpoch)
	}

	return block, nil
}

// payloadLiked checks if the payload of a Block is liked.
func (r *ReferenceProvider) payloadLiked(blockID models.BlockID) (liked bool) {
	engineInstance := r.protocol.Engine()

	block, exists := engineInstance.Tangle.Booker.Block(blockID)
	if !exists {
		return false
	}
	conflictIDs := engineInstance.Tangle.Booker.PayloadConflictIDs(block)

	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict, exists := engineInstance.Ledger.ConflictDAG.Conflict(it.Next())
		if !exists {
			continue
		}
		if !engineInstance.Consensus.ConflictResolver.ConflictLiked(conflict) {
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
