package sybilprotection

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/activitytracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

type SybilProtection struct {
	consensusManaVector *shrinkingmap.ShrinkingMap[identity.ID, int64]
	activityTracker     *activitytracker.ActivityTracker
	validatorSet        *validator.Set
	// attestationsByEpoch stores the blocks issued by a validator per epoch.
	attestationsByEpoch        *memstorage.EpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]]
	storageInstance            *storage.Storage
	lastEvictedEpoch           epoch.Index
	evictionMutex              sync.RWMutex
	optsActivityTrackerOptions []options.Option[activitytracker.ActivityTracker]
}

func New(validatorSet *validator.Set, storageInstance *storage.Storage, timeRetrieverFunc activitytracker.TimeRetrieverFunc, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		consensusManaVector: shrinkingmap.New[identity.ID, int64](),
		validatorSet:        validatorSet,
		attestationsByEpoch: memstorage.NewEpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]](),
		storageInstance:     storageInstance,
	}, opts, func(s *SybilProtection) {
		s.activityTracker = activitytracker.New(validatorSet, timeRetrieverFunc, s.optsActivityTrackerOptions...)
	})
}

func (s *SybilProtection) Attestors(index epoch.Index) (attestors *validator.Set) {
	attestors = validator.NewSet()
	if storageInstance := s.attestationsByEpoch.Get(index, false); storageInstance != nil {
		storageInstance.ForEachKey(func(attestorID identity.ID) bool {
			attestors.Add(validator.New(attestorID, validator.WithWeight(lo.Return1(s.consensusManaVector.Get(attestorID)))))

			return true
		})
	}

	return
}

func (s *SybilProtection) Attestations(index epoch.Index) (attestations []*Attestation) {
	attestations = make([]*Attestation, 0)
	if storageInstance := s.attestationsByEpoch.Get(index, false); storageInstance != nil {
		storageInstance.ForEach(func(_ identity.ID, attestationsStorage *memstorage.Storage[models.BlockID, *Attestation]) bool {
			attestations = append(attestations, lo.Return2(attestationsStorage.First()))

			return true
		})
	}

	return
}

func (s *SybilProtection) AddBlockFromAttestor(block *models.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	attestationsByIssuer, added := s.attestationsByEpoch.Get(block.ID().Index(), true).RetrieveOrCreate(block.IssuerID(), func() *memstorage.Storage[models.BlockID, *Attestation] {
		return memstorage.New[models.BlockID, *Attestation]()
	})

	if added {
		err := s.storageInstance.Attestors.Store(block.ID().Index(), block.IssuerID())
		if err != nil {
			return
		}
	}

	attestationsByIssuer.Set(block.ID(), NewAttestation(block.IssuerID(), block.IssuingTime(), block.Commitment().ID(), lo.PanicOnErr(block.ContentHash()), block.Signature()))
}

func (s *SybilProtection) RemoveBlockFromAttestor(block *models.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if attestationStorage := s.attestationsByEpoch.Get(block.ID().Index(), false); attestationStorage != nil {
		if attestationsByIssuer, exists := attestationStorage.Get(block.IssuerID()); exists {
			if attestationsByIssuer.Delete(block.ID()); attestationsByIssuer.IsEmpty() {
				s.storageInstance.Attestors.Delete(block.ID().Index(), block.IssuerID())
			}
		}
	}
}

func (s *SybilProtection) UpdateConsensusWeights(weightUpdates map[identity.ID]*storageModels.TimedBalance) {
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

func (s *SybilProtection) Evict(index epoch.Index) {
	s.evictionMutex.Lock()
	defer s.evictionMutex.Unlock()

	s.attestationsByEpoch.Evict(index)

	s.lastEvictedEpoch = index
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithActivityTrackerOptions sets the options to be passed to activity manager.
func WithActivityTrackerOptions(activityTrackerOptions ...options.Option[activitytracker.ActivityTracker]) options.Option[SybilProtection] {
	return func(a *SybilProtection) {
		a.optsActivityTrackerOptions = activityTrackerOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
