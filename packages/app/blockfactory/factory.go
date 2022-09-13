package blockfactory

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models/payload"
)

// region Factory ///////////////////////////////////////////////////////////////////////////////////////////////

// Factory acts as a factory to create new blocks.
type Factory struct {
	Events *Events

	identity *identity.LocalIdentity

	selector       TipSelector
	referencesFunc ReferencesFunc
	commitmentFunc CommitmentFunc
	// referenceProvider *ReferenceProvider
}

// NewBlockFactory creates a new block factory.
func NewBlockFactory(localIdentity *identity.LocalIdentity, tipSelector TipSelectorFunc, commitmentFunc CommitmentFunc, opts ...options.Option[Factory]) *Factory {
	// referenceProvider := NewReferenceProvider()

	return options.Apply(&Factory{
		Events:         NewEvents(),
		identity:       localIdentity,
		selector:       tipSelector,
		commitmentFunc: commitmentFunc,
		// referencesFunc:    referenceProvider.References,
		// referenceProvider: referenceProvider,
	}, opts)
}

// IssuePayload creates a new block including sequence number and tip selection and returns it.
func (f *Factory) IssuePayload(p payload.Payload, parentsCount ...int) (*models.Block, error) {
	blk, err := f.issuePayload(p, nil, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("block could not be issued: %w", err))
		return nil, err
	}

	f.Events.BlockConstructed.Trigger(blk)
	return blk, nil
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (f *Factory) IssuePayloadWithReferences(p payload.Payload, references models.ParentBlockIDs, parentsCount ...int) (*models.Block, error) {
	blk, err := f.issuePayload(p, references, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("block could not be issued: %w", err))
		return nil, err
	}

	f.Events.BlockConstructed.Trigger(blk)
	return blk, nil
}

// issuePayload create a new block. If there are any supplied references, it uses them. Otherwise, uses tip selection.
// It also triggers the BlockConstructed event once it's done, which is for example used by the plugins to listen for
// blocks that shall be attached to the tangle.
func (f *Factory) issuePayload(p payload.Payload, references models.ParentBlockIDs, parentsCountOpt ...int) (*models.Block, error) {
	parentsCount := 2
	if len(parentsCountOpt) > 0 {
		parentsCount = parentsCountOpt[0]
	}

	payloadBytes, err := p.Bytes()
	if err != nil {
		return nil, errors.Errorf("could not serialize payload: %w", err)
	}

	if payloadLen := len(payloadBytes); payloadLen > payload.MaxSize {
		return nil, errors.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
	}

	epochCommitment, lastConfirmedEpochIndex, epochCommitmentErr := f.commitmentFunc()
	if epochCommitmentErr != nil {
		err = errors.Errorf("cannot retrieve epoch commitment: %w", epochCommitmentErr)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// select tips, perform PoW and prepare references
	references, nonce, issuingTime, err := f.selectTipsAndPerformPoW(p, references, parentsCount, f.identity.PublicKey(), lastConfirmedEpochIndex, epochCommitment)
	if err != nil {
		return nil, errors.Errorf("could not select tips and perform PoW: %w", err)
	}

	// create the signature
	signature, err := f.sign(references, issuingTime, f.identity.PublicKey(), p, nonce, lastConfirmedEpochIndex, epochCommitment)
	if err != nil {
		return nil, errors.Errorf("signing failed: %w", err)
	}

	block := models.NewBlock(
		models.WithParents(references),
		models.WithIssuer(f.identity.PublicKey()),
		models.WithIssuingTime(issuingTime),
		models.WithSequenceNumber(1337),
		models.WithPayload(p),
	)

	blk, err := models.NewBlockWithValidation(
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
		return nil, errors.Errorf("there is a problem with the block syntax: %w", err)
	}
	_ = blk.DetermineID()

	return blk, nil
}

func (f *Factory) selectTipsAndPerformPoW(p payload.Payload, providedReferences models.ParentBlockIDs, parentsCount int, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, lastConfirmedEpoch epoch.Index, epochCommittment *epoch.ECRecord) (references models.ParentBlockIDs, nonce uint64, issuingTime time.Time, err error) {
	// Perform PoW with given information if there are references provided.
	if !providedReferences.IsEmpty() {
		issuingTime = f.getIssuingTime(providedReferences[models.StrongParentType])
		nonce, err = f.doPOW(providedReferences, issuingTime, issuerPublicKey, sequenceNumber, p, lastConfirmedEpoch, epochCommittment)
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
		for strongParent := range references[models.StrongParentType] {
			delete(references[models.WeakParentType], strongParent)
		}

		// fill up weak references with weak references to liked missing conflicts
		if _, exists := references[models.WeakParentType]; !exists {
			references[models.WeakParentType] = models.NewBlockIDs()
		}
		references[models.WeakParentType].AddAll(f.referenceProvider.ReferencesToMissingConflicts(issuingTime, models.MaxParentsCount-len(references[models.WeakParentType])))

		if len(references[models.WeakParentType]) == 0 {
			delete(references, models.WeakParentType)
		}

		nonce, err = f.doPOW(references, issuingTime, issuerPublicKey, sequenceNumber, p, lastConfirmedEpoch, epochCommittment)
	}

	if err != nil {
		return nil, 0, time.Time{}, errors.Errorf("pow failed: %w", err)
	}

	return references, nonce, issuingTime, nil
}

func (f *Factory) getIssuingTime(parents models.BlockIDs) time.Time {
	issuingTime := time.Now()

	// due to the ParentAge check we must ensure that we set the right issuing time.

	// TODO: when TipManager is ready, we need to check the issuing time of the parents.
	// for parent := range parents {
	//	f.tangle.Storage.Block(parent).Consume(func(blk *tangle.Block) {
	//		if blk.ID() != tangle.EmptyBlockID && !blk.IssuingTime().Before(issuingTime) {
	//			issuingTime = blk.IssuingTime()
	//		}
	//	})
	// }

	return issuingTime
}

func (f *Factory) tips(parentsCount int) (parents models.BlockIDs) {
	parents = f.selector.Tips(parentsCount)

	// TODO: when Ledger is refactored, we need to rework the stuff below
	// tx, ok := p.(utxo.Transaction)
	// if !ok {
	//	return parents
	// }

	// If the block is issuing a transaction and is a double spend, we add it in parallel to the earliest attachment
	// to prevent a double spend from being issued in its past cone.
	// if conflictingTransactions := f.tangle.Ledger.Utils.ConflictingTransactions(tx.ID()); !conflictingTransactions.IsEmpty() {
	//	if earliestAttachment := f.EarliestAttachment(conflictingTransactions); earliestAttachment != nil {
	//		return earliestAttachment.ParentsByType(tangle.StrongParentType)
	//	}
	// }

	return parents
}

func (f *Factory) sign(references models.ParentBlockIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, blockPayload payload.Payload, nonce uint64, latestConfirmedEpoch epoch.Index, epochCommitment *epoch.ECRecord) (ed25519.Signature, error) {
	// create a dummy block to simplify marshaling
	dummy := models.NewBlock(references, issuingTime, key, seq, blockPayload, nonce, ed25519.EmptySignature, latestConfirmedEpoch, epochCommitment)
	dummyBytes, err := dummy.Bytes()
	if err != nil {
		return ed25519.EmptySignature, err
	}

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.identity.Sign(dummyBytes[:contentLength]), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelector //////////////////////////////////////////////////////////////////////////////////////////////////

// A TipSelector selects two tips, parent2 and parent1, for a new block to attach to.
type TipSelector interface {
	Tips(countParents int) (parents models.BlockIDs)
}

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(countParents int) (parents models.BlockIDs)

// Tips calls f().
func (f TipSelectorFunc) Tips(countParents int) (parents models.BlockIDs) {
	return f(countParents)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferencesFunc ///////////////////////////////////////////////////////////////////////////////////////////////////

// ReferencesFunc is a function type that returns like references a given set of parents of a Block.
type ReferencesFunc func(payload payload.Payload, strongParents models.BlockIDs, issuingTime time.Time) (references models.ParentBlockIDs, err error)

// CommitmentFunc is a function type that returns the commitment of the latest committable epoch.
type CommitmentFunc func() (ecRecord *epoch.ECRecord, lastConfirmedEpochIndex epoch.Index, err error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
