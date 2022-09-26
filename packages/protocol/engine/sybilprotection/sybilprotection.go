package sybilprotection

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/activitytracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
)

type SybilProtection struct {
	activityTracker            *activitytracker.ActivityTracker
	validatorSet               *validator.Set
	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
	validatorWeightRetriever   func(issuerID identity.ID) (int64, time.Time, error)
}

func New(validatorSet *validator.Set, retrieverFunc activitytracker.TimeRetrieverFunc, validatorWeightRetriever func(issuerID identity.ID) (int64, time.Time, error), opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		validatorSet:             validatorSet,
		validatorWeightRetriever: validatorWeightRetriever,
	}, opts, func(s *SybilProtection) {
		s.activityTracker = activitytracker.New(validatorSet, retrieverFunc, s.optsActivityTrackerOptions...)
	})
}

func (s *SybilProtection) TrackActiveValidators(block *blockdag.Block) {
	activeValidator, exists := s.validatorSet.Get(block.IssuerID())
	if !exists {
		// TODO: figure out the validator weight here
		weight, _, err := s.validatorWeightRetriever(block.IssuerID())
		if err != nil {
			fmt.Println("error while retrieving weight", err)
			return
		}
		activeValidator = validator.New(block.IssuerID(), validator.WithWeight(weight))
	}

	s.activityTracker.Update(activeValidator, block.IssuingTime())
}

func (s *SybilProtection) AddValidator(issuerID identity.ID, activityTime time.Time) {
	weight, _, err := s.validatorWeightRetriever(issuerID)
	if err != nil {
		fmt.Println("error while retrieving weight", err)
		return
	}
	s.activityTracker.Update(validator.New(issuerID, validator.WithWeight(weight)), activityTime)
}
