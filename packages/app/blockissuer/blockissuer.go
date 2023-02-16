package blockissuer

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer/blockfactory"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
)

// region Factory ///////////////////////////////////////////////////////////////////////////////////////////////

// BlockIssuer contains logic to create and issue blocks.
type BlockIssuer struct {
	Events *Events

	*blockfactory.Factory
	ratesetter.RateSetter
	protocol          *protocol.Protocol
	identity          *identity.LocalIdentity
	referenceProvider *blockfactory.ReferenceProvider
	workerPool        *workerpool.WorkerPool

	optsBlockFactoryOptions    []options.Option[blockfactory.Factory]
	optsIgnoreBootstrappedFlag bool
}

// New creates a new block issuer.
func New(protocol *protocol.Protocol, localIdentity *identity.LocalIdentity, opts ...options.Option[BlockIssuer]) *BlockIssuer {
	return options.Apply(&BlockIssuer{
		Events:     NewEvents(),
		identity:   localIdentity,
		protocol:   protocol,
		workerPool: protocol.Workers.CreatePool("BlockIssuer", 2),
		referenceProvider: blockfactory.NewReferenceProvider(protocol, func() epoch.Index {
			return protocol.Engine().Storage.Settings.LatestCommitment().Index()
		}),
	}, opts, func(i *BlockIssuer) {
		i.Factory = blockfactory.NewBlockFactory(
			localIdentity,
			func(blockID models.BlockID) (block *blockdag.Block, exists bool) {
				return i.protocol.Engine().Tangle.BlockDAG.Block(blockID)
			},
			func(countParents int) (parents models.BlockIDs) {
				return i.protocol.TipManager.Tips(countParents)
			},
			i.referenceProvider.References,
			func() (ecRecord *commitment.Commitment, lastConfirmedEpochIndex epoch.Index, err error) {
				latestCommitment := i.protocol.Engine().Storage.Settings.LatestCommitment()
				confirmedEpochIndex := i.protocol.Engine().Storage.Settings.LatestConfirmedEpoch()

				return latestCommitment, confirmedEpochIndex, nil
			},
			i.optsBlockFactoryOptions...)
	}, (*BlockIssuer).setupEvents)
}

func (i *BlockIssuer) setupEvents() {
}

// IssuePayload creates a new block including sequence number and tip selection, submits it to be processed and returns it.
func (i *BlockIssuer) IssuePayload(p payload.Payload, parentsCount ...int) (block *models.Block, err error) {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return nil, ErrNotBootstraped
	}

	block, err = i.Factory.CreateBlock(p, parentsCount...)
	if err != nil {
		i.Events.Error.Trigger(errors.Wrap(err, "block could not be created"))
		return block, err
	}
	return block, i.issueBlock(block)
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (i *BlockIssuer) IssuePayloadWithReferences(p payload.Payload, references models.ParentBlockIDs, strongParentsCountOpt ...int) (block *models.Block, err error) {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return nil, ErrNotBootstraped
	}

	block, err = i.Factory.CreateBlockWithReferences(p, references, strongParentsCountOpt...)
	if err != nil {
		i.Events.Error.Trigger(errors.Wrap(err, "block with references could not be created"))
		return nil, err
	}

	return block, i.issueBlock(block)
}

func (i *BlockIssuer) issueBlock(block *models.Block) error {
	if err := i.protocol.ProcessBlock(block, i.identity.ID()); err != nil {
		return err
	}
	i.Events.BlockIssued.Trigger(block)

	return nil
}

// IssueBlockAndAwaitBlockToBeBooked awaits maxAwait for the given block to get booked.
func (i *BlockIssuer) IssueBlockAndAwaitBlockToBeBooked(block *models.Block, maxAwait time.Duration) error {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return ErrNotBootstraped
	}

	// first subscribe to the transaction booked event
	booked := make(chan *booker.Block, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)

	hook := i.protocol.Events.Engine.Tangle.Booker.BlockBooked.Hook(func(bookedBlock *booker.Block) {
		if block.ID() != bookedBlock.ID() {
			return
		}
		select {
		case booked <- bookedBlock:
		case <-exit:
		}
	}, event.WithWorkerPool(i.workerPool))
	defer hook.Unhook()

	err := i.issueBlock(block)
	if err != nil {
		return errors.Wrapf(err, "failed to issue block %s", block.ID().String())
	}

	timer := time.NewTimer(maxAwait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ErrBlockWasNotBookedInTime
	case <-booked:
		return nil
	}
}

// IssueBlockAndAwaitBlockToBeScheduled awaits maxAwait for the given block to get issued.
func (i *BlockIssuer) IssueBlockAndAwaitBlockToBeScheduled(block *models.Block, maxAwait time.Duration) error {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return ErrNotBootstraped
	}

	scheduled := make(chan *scheduler.Block, 1)
	exit := make(chan struct{})
	defer close(exit)

	hook := i.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(scheduledBlock *scheduler.Block) {
		if block.ID() != scheduledBlock.ID() {
			return
		}
		select {
		case scheduled <- scheduledBlock:
		case <-exit:
		}
	}, event.WithWorkerPool(i.workerPool))
	defer hook.Unhook()

	err := i.issueBlock(block)
	if err != nil {
		return errors.Wrapf(err, "failed to issue block %s", block.ID().String())
	}

	timer := time.NewTimer(maxAwait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ErrBlockWasNotScheduledInTime
	case <-scheduled:
		return nil
	}
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockFactoryOptions(blockFactoryOptions ...options.Option[blockfactory.Factory]) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.optsBlockFactoryOptions = blockFactoryOptions
	}
}

func WithRateSetter(rateSetter ratesetter.RateSetter) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.RateSetter = rateSetter
	}
}

func WithIgnoreBootstrappedFlag(ignoreBootstrappedFlag bool) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.optsIgnoreBootstrappedFlag = ignoreBootstrappedFlag
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
