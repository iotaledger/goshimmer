package filter

import (
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	ErrorCommitmentNotCommittable      = errors.New("a block cannot commit to an epoch that cannot objectively be committable yet")
	ErrorsBlockTimeTooFarAheadInFuture = errors.New("a block cannot be too far ahead in the future")
	ErrorsInvalidSignature             = errors.New("block has invalid signature")
	ErrorsSignatureValidationFailed    = errors.New("error validating block signature")
)

// Filter filters blocks.
type Filter struct {
	Events *Events

	minCommittableEpochDelta int64

	optsMaxAllowedWallClockDrift time.Duration
	optsMinCommittableEpochAge   time.Duration
	optsSignatureValidation      bool
}

// New creates a new Filter.
func New(opts ...options.Option[Filter]) (inbox *Filter) {
	return options.Apply(&Filter{
		Events:                  NewEvents(),
		optsSignatureValidation: true,
	}, opts, func(f *Filter) {
		f.minCommittableEpochDelta = int64(f.optsMinCommittableEpochAge.Seconds()) / epoch.Duration
	})
}

// ProcessReceivedBlock processes block from the given source.
func (f *Filter) ProcessReceivedBlock(block *models.Block, source identity.ID) {
	// Check if the block is trying to commit to an epoch that is not yet committable
	if f.optsMinCommittableEpochAge > 0 && block.Commitment().Index() > block.ID().Index()-epoch.Index(f.minCommittableEpochDelta) {
		f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
			Block:  block,
			Reason: errors.WithMessagef(ErrorCommitmentNotCommittable, "block at epoch %d committing to epoch %d", block.ID().Index(), block.Commitment().Index()),
		})
		return
	}

	// Verify the timestamp is not too far in the future
	timeDelta := time.Since(block.IssuingTime())
	if timeDelta < -f.optsMaxAllowedWallClockDrift {
		f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
			Block:  block,
			Reason: errors.WithMessagef(ErrorsBlockTimeTooFarAheadInFuture, "issuing time ahead %s vs %s allowed", -timeDelta, f.optsMaxAllowedWallClockDrift),
		})
		return
	}

	if f.optsSignatureValidation {
		// Verify the block signature
		if valid, err := block.VerifySignature(); !valid {
			if err != nil {
				f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
					Block:  block,
					Reason: errors.WithMessagef(ErrorsInvalidSignature, "error: %s", err.Error()),
				})
				return
			}

			f.Events.BlockFiltered.Trigger(&BlockFilteredEvent{
				Block:  block,
				Reason: ErrorsInvalidSignature,
			})
			return
		}
	}

	f.Events.BlockAllowed.Trigger(block)
}

// WithMinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func WithMinCommittableEpochAge(d time.Duration) options.Option[Filter] {
	return func(filter *Filter) {
		filter.optsMinCommittableEpochAge = d
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
