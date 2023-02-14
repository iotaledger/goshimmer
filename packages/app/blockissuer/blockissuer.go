package blockissuer

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/pkg/errors"

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

	optsBlockFactoryOptions            []options.Option[blockfactory.Factory]
	optsIgnoreBootstrappedFlag         bool
	optsTimeSinceConfirmationThreshold time.Duration
}

// New creates a new block issuer.
func New(protocol *protocol.Protocol, localIdentity *identity.LocalIdentity, opts ...options.Option[BlockIssuer]) *BlockIssuer {
	return options.Apply(&BlockIssuer{
		Events:   NewEvents(),
		identity: localIdentity,
		protocol: protocol,
	}, opts, func(i *BlockIssuer) {
		i.referenceProvider = blockfactory.NewReferenceProvider(protocol, i.optsTimeSinceConfirmationThreshold, func() epoch.Index {
			return protocol.Engine().Storage.Settings.LatestCommitment().Index()
		})

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
				if err != nil {
					return nil, 0, err
				}
				confirmedEpochIndex := i.protocol.Engine().Storage.Settings.LatestConfirmedEpoch()
				if err != nil {
					return nil, 0, err
				}

				return latestCommitment, confirmedEpochIndex, nil
			},
			i.optsBlockFactoryOptions...)
	}, (*BlockIssuer).setupEvents)
}

func (i *BlockIssuer) setupEvents() {
	i.Factory.Events.Error.Attach(event.NewClosure(i.Events.Error.Trigger))
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
	err := i.protocol.ProcessBlock(block, i.identity.ID())
	i.Events.BlockIssued.Trigger(block)
	return err
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

	closure := event.NewClosure(func(evt *booker.BlockBookedEvent) {
		if block.ID() != evt.Block.ID() {
			return
		}
		select {
		case booked <- evt.Block:
		case <-exit:
		}
	})
	i.protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(closure)
	defer i.protocol.Events.Engine.Tangle.Booker.BlockBooked.Detach(closure)

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

	closure := event.NewClosure(func(scheduledBlock *scheduler.Block) {
		if block.ID() != scheduledBlock.ID() {
			return
		}
		select {
		case scheduled <- scheduledBlock:
		case <-exit:
		}
	})
	i.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(closure)
	defer i.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Detach(closure)

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

// WithTimeSinceConfirmationThreshold returns an option that sets the time since confirmation threshold.
func WithTimeSinceConfirmationThreshold(timeSinceConfirmationThreshold time.Duration) options.Option[BlockIssuer] {
	return func(o *BlockIssuer) {
		o.optsTimeSinceConfirmationThreshold = timeSinceConfirmationThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
