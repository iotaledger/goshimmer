package tangleold

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/node/clock"
)

const storeSequenceInterval = 100

// region BlockFactory ///////////////////////////////////////////////////////////////////////////////////////////////

// BlockFactory acts as a factory to create new blocks.
type BlockFactory struct {
	Events *BlockFactoryEvents

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

// NewBlockFactory creates a new block factory.
func NewBlockFactory(tangle *Tangle, selector TipSelector, referencesFunc ...ReferencesFunc) *BlockFactory {
	sequence, err := kvstore.NewSequence(tangle.Options.Store, []byte(DBSequenceNumber), storeSequenceInterval)
	if err != nil {
		panic(fmt.Sprintf("could not create block sequence number: %v", err))
	}

	referenceProvider := NewReferenceProvider(tangle)
	f := referenceProvider.References
	if len(referencesFunc) != 0 {
		f = referencesFunc[0]
	}

	return &BlockFactory{
		Events:            NewBlockFactoryEvents(),
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

// SetWorker sets the PoW worker to be used for the blocks.
func (f *BlockFactory) SetWorker(worker Worker) {
	f.workerMutex.Lock()
	defer f.workerMutex.Unlock()
	f.worker = worker
}

// SetTimeout sets the timeout for PoW.
func (f *BlockFactory) SetTimeout(timeout time.Duration) {
	f.powTimeout = timeout
}

// IssuePayload creates a new block including sequence number and tip selection and returns it.
func (f *BlockFactory) IssuePayload(p payload.Payload, parentsCount ...int) (*Block, error) {
	blk, err := f.issuePayload(p, nil, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("block could not be issued: %w", err))
		return nil, err
	}

	f.Events.BlockConstructed.Trigger(&BlockConstructedEvent{blk})
	return blk, nil
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (f *BlockFactory) IssuePayloadWithReferences(p payload.Payload, references ParentBlockIDs, parentsCount ...int) (*Block, error) {
	blk, err := f.issuePayload(p, references, parentsCount...)
	if err != nil {
		f.Events.Error.Trigger(errors.Errorf("block could not be issued: %w", err))
		return nil, err
	}

	f.Events.BlockConstructed.Trigger(&BlockConstructedEvent{blk})
	return blk, nil
}

// issuePayload create a new block. If there are any supplied references, it uses them. Otherwise, uses tip selection.
// It also triggers the BlockConstructed event once it's done, which is for example used by the plugins to listen for
// blocks that shall be attached to the tangle.
func (f *BlockFactory) issuePayload(p payload.Payload, references ParentBlockIDs, parentsCountOpt ...int) (*Block, error) {
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

	epochCommitment, lastConfirmedEpochIndex, epochCommitmentErr := f.tangle.Options.CommitmentFunc()
	if epochCommitmentErr != nil {
		err = errors.Errorf("cannot retrieve epoch commitment: %w", epochCommitmentErr)
		f.Events.Error.Trigger(err)
		return nil, err
	}

	// select tips, perform PoW and prepare references
	references, nonce, issuingTime, err := f.selectTipsAndPerformPoW(p, references, parentsCount, issuerPublicKey, sequenceNumber, lastConfirmedEpochIndex, epochCommitment)
	if err != nil {
		return nil, errors.Errorf("could not select tips and perform PoW: %w", err)
	}

	// create the signature
	signature, err := f.sign(references, issuingTime, issuerPublicKey, sequenceNumber, p, nonce, lastConfirmedEpochIndex, epochCommitment)
	if err != nil {
		return nil, errors.Errorf("signing failed: %w", err)
	}

	blk, err := NewBlockWithValidation(
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

func (f *BlockFactory) selectTipsAndPerformPoW(p payload.Payload, providedReferences ParentBlockIDs, parentsCount int, issuerPublicKey ed25519.PublicKey, sequenceNumber uint64, lastConfirmedEpoch epoch.Index, epochCommittment *epoch.ECRecord) (references ParentBlockIDs, nonce uint64, issuingTime time.Time, err error) {
	// Perform PoW with given information if there are references provided.
	if !providedReferences.IsEmpty() {
		issuingTime = f.getIssuingTime(providedReferences[StrongParentType])
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
		for strongParent := range references[StrongParentType] {
			delete(references[WeakParentType], strongParent)
		}

		// fill up weak references with weak references to liked missing conflicts
		if _, exists := references[WeakParentType]; !exists {
			references[WeakParentType] = NewBlockIDs()
		}
		references[WeakParentType].AddAll(f.ReferenceProvider.ReferencesToMissingConflicts(issuingTime, MaxParentsCount-len(references[WeakParentType])))

		if len(references[WeakParentType]) == 0 {
			delete(references, WeakParentType)
		}

		nonce, err = f.doPOW(references, issuingTime, issuerPublicKey, sequenceNumber, p, lastConfirmedEpoch, epochCommittment)
	}

	if err != nil {
		return nil, 0, time.Time{}, errors.Errorf("pow failed: %w", err)
	}

	return references, nonce, issuingTime, nil
}

func (f *BlockFactory) getIssuingTime(parents BlockIDs) time.Time {
	issuingTime := clock.SyncedTime()

	// due to the ParentAge check we must ensure that we set the right issuing time.

	for parent := range parents {
		f.tangle.Storage.Block(parent).Consume(func(blk *Block) {
			if blk.ID() != EmptyBlockID && !blk.IssuingTime().Before(issuingTime) {
				issuingTime = blk.IssuingTime()
			}
		})
	}

	return issuingTime
}

func (f *BlockFactory) tips(p payload.Payload, parentsCount int) (parents BlockIDs) {
	parents = f.selector.Tips(p, parentsCount)

	tx, ok := p.(utxo.Transaction)
	if !ok {
		return parents
	}

	// If the block is issuing a transaction and is a double spend, we add it in parallel to the earliest attachment
	// to prevent a double spend from being issued in its past cone.
	if conflictingTransactions := f.tangle.Ledger.Utils.ConflictingTransactions(tx.ID()); !conflictingTransactions.IsEmpty() {
		if earliestAttachment := f.EarliestAttachment(conflictingTransactions); earliestAttachment != nil {
			return earliestAttachment.ParentsByType(StrongParentType)
		}
	}

	return parents
}

func (f *BlockFactory) EarliestAttachment(transactionIDs utxo.TransactionIDs, earliestAttachmentMustBeBooked ...bool) (earliestAttachment *Block) {
	var earliestIssuingTime time.Time
	for it := transactionIDs.Iterator(); it.HasNext(); {
		f.tangle.Storage.Attachments(it.Next()).Consume(func(attachment *Attachment) {
			f.tangle.Storage.Block(attachment.BlockID()).Consume(func(block *Block) {
				f.tangle.Storage.BlockMetadata(attachment.BlockID()).Consume(func(blockMetadata *BlockMetadata) {
					if ((len(earliestAttachmentMustBeBooked) > 0 && !earliestAttachmentMustBeBooked[0]) || blockMetadata.IsBooked()) &&
						(earliestAttachment == nil || block.IssuingTime().Before(earliestIssuingTime)) {
						earliestAttachment = block
						earliestIssuingTime = block.IssuingTime()
					}
				})
			})
		})
	}

	return earliestAttachment
}

func (f *BlockFactory) LatestAttachment(transactionID utxo.TransactionID) (latestAttachment *Block) {
	var latestIssuingTime time.Time
	f.tangle.Storage.Attachments(transactionID).Consume(func(attachment *Attachment) {
		f.tangle.Storage.Block(attachment.BlockID()).Consume(func(block *Block) {
			f.tangle.Storage.BlockMetadata(attachment.BlockID()).Consume(func(blockMetadata *BlockMetadata) {
				if blockMetadata.IsBooked() && block.IssuingTime().After(latestIssuingTime) {
					latestAttachment = block
					latestIssuingTime = block.IssuingTime()
				}
			})
		})
	})

	return latestAttachment
}

// Shutdown closes the BlockFactory and persists the sequence number.
func (f *BlockFactory) Shutdown() {
	if err := f.sequence.Release(); err != nil {
		f.Events.Error.Trigger(fmt.Errorf("could not release block sequence number: %w", err))
	}
}

// doPOW performs pow on the block and returns a nonce.
func (f *BlockFactory) doPOW(references ParentBlockIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, blockPayload payload.Payload, latestConfirmedEpoch epoch.Index, epochCommitment *epoch.ECRecord) (uint64, error) {
	// create a dummy block to simplify marshaling
	block := NewBlock(references, issuingTime, key, seq, blockPayload, 0, ed25519.EmptySignature, latestConfirmedEpoch, epochCommitment)
	dummy, err := block.Bytes()
	if err != nil {
		return 0, err
	}

	f.workerMutex.RLock()
	defer f.workerMutex.RUnlock()
	return f.worker.DoPOW(dummy)
}

func (f *BlockFactory) sign(references ParentBlockIDs, issuingTime time.Time, key ed25519.PublicKey, seq uint64, blockPayload payload.Payload, nonce uint64, latestConfirmedEpoch epoch.Index, epochCommitment *epoch.ECRecord) (ed25519.Signature, error) {
	// create a dummy block to simplify marshaling
	dummy := NewBlock(references, issuingTime, key, seq, blockPayload, nonce, ed25519.EmptySignature, latestConfirmedEpoch, epochCommitment)
	dummyBytes, err := dummy.Bytes()
	if err != nil {
		return ed25519.EmptySignature, err
	}

	contentLength := len(dummyBytes) - len(dummy.Signature())
	return f.localIdentity.Sign(dummyBytes[:contentLength]), nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelector //////////////////////////////////////////////////////////////////////////////////////////////////

// A TipSelector selects two tips, parent2 and parent1, for a new block to attach to.
type TipSelector interface {
	Tips(p payload.Payload, countParents int) (parents BlockIDs)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipSelectorFunc //////////////////////////////////////////////////////////////////////////////////////////////

// The TipSelectorFunc type is an adapter to allow the use of ordinary functions as tip selectors.
type TipSelectorFunc func(p payload.Payload, countParents int) (parents BlockIDs)

// Tips calls f().
func (f TipSelectorFunc) Tips(p payload.Payload, countParents int) (parents BlockIDs) {
	return f(p, countParents)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Worker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// A Worker performs the PoW for the provided block in serialized byte form.
type Worker interface {
	DoPOW([]byte) (nonce uint64, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WorkerFunc ///////////////////////////////////////////////////////////////////////////////////////////////////

// The WorkerFunc type is an adapter to allow the use of ordinary functions as a PoW performer.
type WorkerFunc func([]byte) (uint64, error)

// DoPOW calls f(blk).
func (f WorkerFunc) DoPOW(blk []byte) (uint64, error) {
	return f(blk)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ZeroWorker ///////////////////////////////////////////////////////////////////////////////////////////////////

// ZeroWorker is a PoW worker that always returns 0 as the nonce.
var ZeroWorker = WorkerFunc(func([]byte) (uint64, error) { return 0, nil })

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region PrepareLikeReferences ///////////////////////////////////////////////////////////////////////////////////////////////////

// ReferencesFunc is a function type that returns like references a given set of parents of a Block.
type ReferencesFunc func(payload payload.Payload, strongParents BlockIDs, issuingTime time.Time) (references ParentBlockIDs, err error)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
