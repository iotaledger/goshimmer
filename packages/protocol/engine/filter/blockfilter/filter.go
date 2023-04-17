package blockfilter

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

var (
	ErrorCommitmentNotCommittable      = errors.New("a block cannot commit to a slot that cannot objectively be committable yet")
	ErrorsBlockTimeTooFarAheadInFuture = errors.New("a block cannot be too far ahead in the future")
	ErrorsInvalidSignature             = errors.New("block has invalid signature")
	ErrorsSignatureValidationFailed    = errors.New("error validating block signature")
)

// Filter filters blocks.
type Filter struct {
	events *filter.Events

	optsMaxAllowedWallClockDrift time.Duration
	optsMinCommittableSlotAge    slot.Index
	optsSignatureValidation      bool

	module.Module
}

func NewProvider(opts ...options.Option[Filter]) module.Provider[*engine.Engine, filter.Filter] {
	return module.Provide(func(e *engine.Engine) filter.Filter {
		f := New(opts...)

		e.HookConstructed(func() {
			f.events.BlockFiltered.Hook(func(filteredEvent *filter.BlockFilteredEvent) {
				e.Events.Error.Trigger(errors.Wrapf(filteredEvent.Reason, "block (%s) filtered", filteredEvent.Block.ID()))
			}, event.WithWorkerPool(e.Workers.CreatePool("Filter", 2)))

			e.Events.Filter.LinkTo(f.events)
		})

		return f
	})
}

var _ filter.Filter = new(Filter)

// New creates a new Filter.
func New(opts ...options.Option[Filter]) *Filter {
	return options.Apply(&Filter{
		events:                  filter.NewEvents(),
		optsSignatureValidation: true,
	}, opts,
		(*Filter).TriggerConstructed,
		(*Filter).TriggerInitialized,
	)
}

func (f *Filter) Events() *filter.Events {
	return f.events
}

// ProcessReceivedBlock processes block from the given source.
func (f *Filter) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	// Check if the block is trying to commit to a slot that is not yet committable
	if f.optsMinCommittableSlotAge > 0 && block.Commitment().Index() > 0 && block.Commitment().Index() > block.ID().Index()-f.optsMinCommittableSlotAge {
		f.events.BlockFiltered.Trigger(&filter.BlockFilteredEvent{
			Block:  block,
			Reason: errors.WithMessagef(ErrorCommitmentNotCommittable, "block at slot %d committing to slot %d", block.ID().Index(), block.Commitment().Index()),
		})
		return
	}

	// Verify the timestamp is not too far in the future
	timeDelta := time.Since(block.IssuingTime())
	if timeDelta < -f.optsMaxAllowedWallClockDrift {
		f.events.BlockFiltered.Trigger(&filter.BlockFilteredEvent{
			Block:  block,
			Reason: errors.WithMessagef(ErrorsBlockTimeTooFarAheadInFuture, "issuing time ahead %s vs %s allowed", -timeDelta, f.optsMaxAllowedWallClockDrift),
		})
		return
	}

	if f.optsSignatureValidation {
		// Verify the block signature
		if valid, err := block.VerifySignature(); !valid {
			if err != nil {
				f.events.BlockFiltered.Trigger(&filter.BlockFilteredEvent{
					Block:  block,
					Reason: errors.WithMessagef(ErrorsSignatureValidationFailed, "error: %s", err.Error()),
				})
				return
			}

			f.events.BlockFiltered.Trigger(&filter.BlockFilteredEvent{
				Block:  block,
				Reason: ErrorsInvalidSignature,
			})
			return
		}
	}

	f.events.BlockAllowed.Trigger(block)
}

// WithMinCommittableSlotAge specifies how old a slot has to be for it to be committable.
func WithMinCommittableSlotAge(age slot.Index) options.Option[Filter] {
	return func(filter *Filter) {
		filter.optsMinCommittableSlotAge = age
	}
}

// WithMaxAllowedWallClockDrift specifies how far in the future are blocks allowed to be ahead of our own wall clock (defaults to 0 seconds).
func WithMaxAllowedWallClockDrift(d time.Duration) options.Option[Filter] {
	return func(filter *Filter) {
		filter.optsMaxAllowedWallClockDrift = d
	}
}

// WithSignatureValidation specifies if the block signature should be validated (defaults to yes).
func WithSignatureValidation(validation bool) options.Option[Filter] {
	return func(filter *Filter) {
		filter.optsSignatureValidation = validation
	}
}
