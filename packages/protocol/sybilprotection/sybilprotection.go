package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/sybilprotection/activitytracker"
)

type SybilProtection struct {
	ActivityTracker *activitytracker.ActivityTracker
	ValidatorSet    *validator.Set

	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
}

func New(timeRetrieverFunc activitytracker.TimeRetrieverFunc, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		ValidatorSet: validator.NewSet(),
	}, opts, func(s *SybilProtection) {
		s.ActivityTracker = activitytracker.New(s.ValidatorSet, timeRetrieverFunc, s.optsActivityTrackerOptions...)
	})
}
