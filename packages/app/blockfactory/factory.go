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

	// referenceProvider *ReferenceProvider
	identity *identity.LocalIdentity

	tipSelector    TipSelector
	referencesFunc ReferencesFunc
	commitmentFunc CommitmentFunc

	optsTipSelectionTimeout       time.Duration
	optsTipSelectionRetryInterval time.Duration
}

// NewBlockFactory creates a new block factory.
func NewBlockFactory(localIdentity *identity.LocalIdentity, tipSelector TipSelectorFunc, commitmentFunc CommitmentFunc, opts ...options.Option[Factory]) *Factory {
	// referenceProvider := NewReferenceProvider()

	return options.Apply(&Factory{
		Events:         NewEvents(),
		identity:       localIdentity,
		tipSelector:    tipSelector,
		commitmentFunc: commitmentFunc,
		// referencesFunc:    referenceProvider.References,
		// referenceProvider: referenceProvider,
		optsTipSelectionTimeout:       10 * time.Second,
		optsTipSelectionRetryInterval: 200 * time.Millisecond,
	}, opts)
}

// IssuePayload creates a new block including sequence number and tip selection and returns it.
func (f *Factory) IssuePayload(p payload.Payload, parentsCount ...int) (*models.Block, error) {
	return f.IssuePayloadWithReferences(p, nil, parentsCount...)
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (f *Factory) IssuePayloadWithReferences(p payload.Payload, references models.ParentBlockIDs, strongParentsCountOpt ...int) (*models.Block, error) {
	strongParentsCount := 2
	if len(strongParentsCountOpt) > 0 {
		strongParentsCount = strongParentsCountOpt[0]
	}

	block, err := f.issuePayload(p, references, strongParentsCount)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("block could not be issued: %w", err))
		return nil, err
	}

	f.Events.BlockConstructed.Trigger(block)
	return block, nil
}

// issuePayload create a new block. If there are any supplied references, it uses them. Otherwise, uses tip selection.
// It also triggers the BlockConstructed event once it's done, which is for example used by the plugins to listen for
// blocks that shall be attached to the tangle.
func (f *Factory) issuePayload(p payload.Payload, references models.ParentBlockIDs, strongParentsCount int) (*models.Block, error) {
	payloadBytes, err := p.Bytes()
	if err != nil {
		return nil, errors.Errorf("could not serialize payload: %w", err)
	}

	if payloadLen := len(payloadBytes); payloadLen > payload.MaxSize {
		return nil, errors.Errorf("maximum payload size of %d bytes exceeded", payloadLen)
	}

	epochCommitment, lastConfirmedEpochIndex, err := f.commitmentFunc()
	if err != nil {
		return nil, errors.Errorf("cannot retrieve epoch commitment: %w", err)
	}

	var issuingTime time.Time
	if references.IsEmpty() {
		references, issuingTime, err = f.tryGetReferences(p, strongParentsCount)
		if err != nil {
			return nil, errors.Errorf("error while trying to get references: %w", err)
		}
	}

	block := models.NewBlock(
		models.WithParents(references),
		models.WithIssuer(f.identity.PublicKey()),
		models.WithIssuingTime(issuingTime),
		models.WithPayload(p),
		models.WithLatestConfirmedEpoch(lastConfirmedEpochIndex),
		models.WithECRecord(epochCommitment),
		models.WithSignature(ed25519.EmptySignature), // placeholder will be set after signing

		// jUsT4fUn
		models.WithSequenceNumber(1337),
		models.WithNonce(42),
	)

	// create the signature
	signature, err := f.sign(block)
	if err != nil {
		return nil, errors.Errorf("signing failed: %w", err)
	}
	block.SetSignature(signature)

	if err = block.DetermineID(); err != nil {
		return nil, errors.Errorf("there is a problem with the block syntax: %w", err)
	}

	return block, nil
}

func (f *Factory) tryGetReferences(p payload.Payload, parentsCount int) (references models.ParentBlockIDs, issuingTime time.Time, err error) {
	references, issuingTime, err = f.getReferences(p, parentsCount)
	if err == nil {
		return references, issuingTime, nil
	}
	f.Events.Error.Trigger(errors.Errorf("could not get references: %w", err))

	timeout := time.NewTimer(f.optsTipSelectionTimeout)
	interval := time.NewTicker(f.optsTipSelectionRetryInterval)
	for {
		select {
		case <-interval.C:
			references, issuingTime, err = f.getReferences(p, parentsCount)
			if err != nil {
				f.Events.Error.Trigger(errors.Errorf("could not get references: %w", err))
				continue
			}

			return references, issuingTime, nil
		case <-timeout.C:
			return nil, time.Time{}, errors.Errorf("timeout while trying to select tips and determine references")
		}
	}
}

func (f *Factory) getReferences(p payload.Payload, parentsCount int) (references models.ParentBlockIDs, issuingTime time.Time, err error) {
	strongParents := f.tips(p, parentsCount)
	if len(strongParents) == 0 {
		return nil, time.Time{}, errors.Errorf("no strong parents were selected in tip selection")
	}
	issuingTime = f.getIssuingTime(strongParents)

	// TODO: remove
	return models.NewParentBlockIDs().AddAll(models.StrongParentType, strongParents), issuingTime, nil
	references, err = f.referencesFunc(p, strongParents, issuingTime)
	// If none of the strong parents are possible references, we have to try again.
	if err != nil {
		return nil, time.Time{}, errors.Errorf("references could not be created: %w", err)
	}

	// Make sure that there's no duplicate between strong and weak parents.
	for strongParent := range references[models.StrongParentType] {
		delete(references[models.WeakParentType], strongParent)
	}

	// fill up weak references with weak references to liked missing conflicts
	if _, exists := references[models.WeakParentType]; !exists {
		references[models.WeakParentType] = models.NewBlockIDs()
	}
	// TODO:
	// references[models.WeakParentType].AddAll(f.referenceProvider.ReferencesToMissingConflicts(issuingTime, models.MaxParentsCount-len(references[models.WeakParentType])))

	if len(references[models.WeakParentType]) == 0 {
		delete(references, models.WeakParentType)
	}

	return references, issuingTime, nil
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

func (f *Factory) tips(p payload.Payload, parentsCount int) (parents models.BlockIDs) {
	parents = f.tipSelector.Tips(parentsCount)

	// TODO: when Ledger is refactored, we need to rework the stuff below
	// tx, ok := p.(utxo.Transaction)
	// if !ok {
	// 	return parents
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

func (f *Factory) sign(block *models.Block) (ed25519.Signature, error) {
	bytes, err := block.Bytes()
	if err != nil {
		return ed25519.EmptySignature, err
	}

	contentLength := len(bytes) - len(block.Signature())
	return f.identity.Sign(bytes[:contentLength]), nil
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

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTipSelectionTimeout(timeout time.Duration) options.Option[Factory] {
	return func(factory *Factory) {
		factory.optsTipSelectionTimeout = timeout
	}
}

func WithTipSelectionRetryInterval(interval time.Duration) options.Option[Factory] {
	return func(factory *Factory) {
		factory.optsTipSelectionRetryInterval = interval
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
