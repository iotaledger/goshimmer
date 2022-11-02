package sybilprotection

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/activitytracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

type SybilProtection struct {
	consensusManaVector        *shrinkingmap.ShrinkingMap[identity.ID, int64]
	activityTracker            *activitytracker.ActivityTracker
	validatorSet               *validator.Set
	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
}

func New(validatorSet *validator.Set, retrieverFunc activitytracker.TimeRetrieverFunc, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		consensusManaVector: shrinkingmap.New[identity.ID, int64](),
		validatorSet:        validatorSet,
	}, opts, func(s *SybilProtection) {
		s.activityTracker = activitytracker.New(validatorSet, retrieverFunc, s.optsActivityTrackerOptions...)
	})
}

func (s *SybilProtection) UpdateConsensusWeights(weightUpdates map[identity.ID]*models.TimedBalance) {
	for id, updateMana := range weightUpdates {
		if updateMana.Balance <= 0 {
			s.consensusManaVector.Delete(id)
		} else {
			s.consensusManaVector.Set(id, updateMana.Balance)
		}
	}
}

func (s *SybilProtection) TrackActiveValidators(block *blockdag.Block) {
	activeValidator, exists := s.validatorSet.Get(block.IssuerID())
	if !exists {
		weight, exists := s.consensusManaVector.Get(block.IssuerID())
		if !exists {
			return
		}
		activeValidator = validator.New(block.IssuerID(), validator.WithWeight(weight))
		s.validatorSet.Add(activeValidator)
	}

	s.activityTracker.Update(activeValidator, block.IssuingTime())
}

func (s *SybilProtection) AddValidator(issuerID identity.ID, activityTime time.Time) {
	weight, exists := s.consensusManaVector.Get(issuerID)
	if !exists {
		fmt.Println("AddValidator: error while retrieving weight", issuerID)
		return
	}
	s.activityTracker.Update(validator.New(issuerID, validator.WithWeight(weight)), activityTime)
}

func (s *SybilProtection) Weights() map[identity.ID]int64 {
	return s.consensusManaVector.AsMap()
}

func (s *SybilProtection) Weight(id identity.ID) (weight int64, exists bool) {
	return s.consensusManaVector.Get(id)
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithActivityTrackerOptions sets the options to be passed to activity manager.
func WithActivityTrackerOptions(activityTrackerOptions ...options.Option[activitytracker.ActivityTracker]) options.Option[SybilProtection] {
	return func(a *SybilProtection) {
		a.optsActivityTrackerOptions = activityTrackerOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
