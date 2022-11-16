package sybilprotection

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/activenodes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

type SybilProtection struct {
	weights      *memstorage.Storage[identity.ID, int64]
	weightsMutex sync.RWMutex
	activeNodes  *activenodes.ActiveNodes
	// attestationsByEpoch stores the blocks issued by a validator per epoch.

	attestationsByEpoch *memstorage.Storage[epoch.Index, *EpochAttestations]
	storage             *storage.Storage
	evictionMutex       sync.RWMutex
}

func New(activeNodes *activenodes.ActiveNodes, storageInstance *storage.Storage) (sybilProtection *SybilProtection) {
	sybilProtection = &SybilProtection{
		activeNodes:         activeNodes,
		weights:             memstorage.New[identity.ID, int64](),
		attestationsByEpoch: memstorage.New[epoch.Index, *EpochAttestations](),
		storage:             storageInstance,
	}

	sybilProtection.importValidators()

	return
}

func (s *SybilProtection) EpochAttestations(index epoch.Index) (epochAttestations *EpochAttestations) {
	return lo.Return1(s.attestationsByEpoch.Get(index))
}

func (s *SybilProtection) HandleSolidBlock(block *blockdag.Block) {
	activeValidator, exists := s.activeNodes.Get(block.IssuerID())
	if !exists {
		weight, exists := s.weights.Get(block.IssuerID())
		if !exists {
			return
		}
		activeValidator = validator.New(block.IssuerID(), validator.WithWeight(weight))
	}

	s.activeNodes.Set(activeValidator, block.IssuingTime())
}

func (s *SybilProtection) HandleAcceptedBlock(block *models.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	lo.Return1(s.attestationsByEpoch.RetrieveOrCreate(block.ID().Index(), func() *EpochAttestations {
		return NewEpochAttestations(s.weights)
	})).Add(block)
}

func (s *SybilProtection) HandledOrphanedBlock(block *models.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	if attestationStorage, exists := s.attestationsByEpoch.Get(block.ID().Index()); exists {
		attestationStorage.Delete(block)
	}
}

func (s *SybilProtection) UpdateConsensusWeights(weightUpdates map[identity.ID]*storageModels.TimedBalance) {
	for id, updateMana := range weightUpdates {
		if updateMana.Balance <= 0 {
			s.weights.Delete(id)
		} else {
			s.weights.Set(id, updateMana.Balance)
		}
	}
}

func (s *SybilProtection) Weights() map[identity.ID]int64 {
	return s.weights.AsMap()
}

func (s *SybilProtection) Weight(id identity.ID) (weight int64, exists bool) {
	return s.weights.Get(id)
}

func (s *SybilProtection) HandleEpochEvicted(index epoch.Index) {
	s.evictionMutex.Lock()
	defer s.evictionMutex.Unlock()

	s.attestationsByEpoch.Delete(index)
}

func (s *SybilProtection) importValidators() {
	snapshotEpoch := s.storage.Settings.LatestCommitment().Index()
	for it := s.storage.Attestors.LoadAll(snapshotEpoch).Iterator(); it.HasNext(); {
		issuerID := it.Next()
		weight, exists := s.weights.Get(issuerID)
		if !exists {
			return
		}
		s.activeNodes.Set(validator.New(issuerID, validator.WithWeight(weight)), snapshotEpoch.EndTime())
	}
}
