package blockissuer

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer/blockfactory"
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
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

	optsRateSetterMode         ratesetter.RateSetterModeType
	optsBlockFactoryOptions    []options.Option[blockfactory.Factory]
	optsIgnoreBootstrappedFlag bool
}

// New creates a new block issuer.
func New(protocol *protocol.Protocol, localIdentity *identity.LocalIdentity, opts ...options.Option[BlockIssuer]) *BlockIssuer {
	return options.Apply(&BlockIssuer{
		Events:   NewEvents(),
		identity: localIdentity,
		protocol: protocol,
		referenceProvider: blockfactory.NewReferenceProvider(func() *engine.Engine { return protocol.Engine() }, func() epoch.Index {
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
	i.RateSetter.Events().BlockIssued.Attach(event.NewClosure[*models.Block](func(block *models.Block) {
		i.protocol.ProcessBlock(block, i.identity.ID())
	}))
}

// IssuePayload creates a new block including sequence number and tip selection and returns it.
func (i *BlockIssuer) IssuePayload(p payload.Payload, parentsCount ...int) (block *models.Block, err error) {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return nil, ErrNotBootstraped
	}

	block, err = i.Factory.CreateBlock(p, parentsCount...)
	if err != nil {
		i.Events.Error.Trigger(errors.Errorf("block could not be created: %w", err))
		return block, err
	}

	return block, i.RateSetter.IssueBlock(block)
}

// IssuePayloadWithReferences creates a new block with the references submit.
func (i *BlockIssuer) IssuePayloadWithReferences(p payload.Payload, references models.ParentBlockIDs, strongParentsCountOpt ...int) (block *models.Block, err error) {
	if !i.optsIgnoreBootstrappedFlag && !i.protocol.Engine().IsBootstrapped() {
		return nil, ErrNotBootstraped
	}

	block, err = i.Factory.CreateBlockWithReferences(p, references, strongParentsCountOpt...)
	if err != nil {
		i.Events.Error.Trigger(errors.Errorf("block with references could not be created: %w", err))
		return nil, err
	}

	return block, i.RateSetter.IssueBlock(block)
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

	closure := event.NewClosure(func(bookedBlock *booker.Block) {
		if block.ID() != bookedBlock.ID() {
			return
		}
		select {
		case booked <- bookedBlock:
		case <-exit:
		}
	})
	i.protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(closure)
	defer i.protocol.Events.Engine.Tangle.Booker.BlockBooked.Detach(closure)

	err := i.RateSetter.IssueBlock(block)

	if err != nil {
		return errors.Errorf("failed to issue block %s: %w", block.ID().String(), err)
	}

	select {
	case <-time.After(maxAwait):
		return ErrBlockWasNotBookedInTime
	case <-booked:
		return nil
	}
}

// IssueBlockAndAwaitBlockToBeIssued awaits maxAwait for the given block to get issued.
func (i *BlockIssuer) IssueBlockAndAwaitBlockToBeIssued(block *models.Block, maxAwait time.Duration) error {
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

	err := i.RateSetter.IssueBlock(block)

	if err != nil {
		return errors.Errorf("failed to issue block %s: %w", block.ID().String(), err)
	}

	select {
	case <-time.After(maxAwait):
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
func WithRateSetterMode(rateSetterMode ratesetter.RateSetterModeType) options.Option[BlockIssuer] {
	return func(issuer *BlockIssuer) {
		issuer.optsRateSetterMode = rateSetterMode
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
