package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/activitytracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
)

type SybilProtection struct {
	activityTracker            *activitytracker.ActivityTracker
	validatorSet               *validator.Set
	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
}

func New(validatorSet *validator.Set, retrieverFunc activitytracker.TimeRetrieverFunc, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		validatorSet: validatorSet,
	}, opts, func(s *SybilProtection) {
		s.activityTracker = activitytracker.New(validatorSet, retrieverFunc, s.optsActivityTrackerOptions...)
	})
}

func (s *SybilProtection) TrackActiveValidators(block *blockdag.Block) {
	activeValidator, exists := s.validatorSet.Get(block.IssuerID())
	if !exists {
		// TODO: figure out the validator weight here
		activeValidator = validator.New(block.IssuerID(), validator.WithWeight(0))
	}

	s.activityTracker.Update(activeValidator, block.IssuingTime())
}
