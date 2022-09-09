package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/sybilprotection/activitytracker"
)

type SybilProtection struct {
	ActivityTracker *activitytracker.ActivityTracker
	Engine          *engine.Engine
	ValidatorSet    *validator.Set

	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
}

func New(engine *engine.Engine, validatorSet *validator.Set, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		ValidatorSet: validatorSet,
	}, opts, func(s *SybilProtection) {
		s.ActivityTracker = activitytracker.New(s.ValidatorSet, engine.Clock.RelativeAcceptedTime, s.optsActivityTrackerOptions...)
	}, (*SybilProtection).setupEvents)
}

func (s *SybilProtection) setupEvents() {
	s.Engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		activeValidator, exists := s.ValidatorSet.Get(block.IssuerID())
		if !exists {
			// TODO: figure out the validator weight here
			activeValidator = validator.New(block.IssuerID(), validator.WithWeight(0))
		}

		s.ActivityTracker.Update(activeValidator, block.IssuingTime())
	}))
}
