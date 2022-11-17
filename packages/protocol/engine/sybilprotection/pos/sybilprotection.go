package pos

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

type SybilProtection struct {
	engine       *engine.Engine
	weights      *memstorage.Storage[identity.ID, int64]
	weightsMutex sync.RWMutex
	activeNodes  *ActiveValidators
	// attestationsByEpoch stores the blocks issued by a validator per epoch.

	attestationsByEpoch *memstorage.Storage[epoch.Index, *Attestations]
	evictionMutex       sync.RWMutex
}

func (s *SybilProtection) WeightsVector() (weightedActors sybilprotection.WeightsVector) {
	// TODO implement me
	panic("implement me")
}

func (s *SybilProtection) Attestations(index epoch.Index) (attestations sybilprotection.Attestations) {
	// TODO implement me
	panic("implement me")
}

// impl.NewActiveNodes(e.Clock.RelativeAcceptedTime, e.optsActiveNodesOptions...)

func New(engineInstance *engine.Engine, opts ...options.Option[SybilProtection]) (sybilProtection *SybilProtection) {
	return options.Apply(&SybilProtection{
		engine:              engineInstance,
		activeNodes:         NewActiveNodes(nil),
		weights:             memstorage.New[identity.ID, int64](),
		attestationsByEpoch: memstorage.New[epoch.Index, *Attestations](),
	}, opts, (*SybilProtection).importValidators)
}

func Provider(opts ...options.Option[SybilProtection]) func(engine *engine.Engine) sybilprotection.SybilProtection {
	var (
		singleton           *SybilProtection
		createSingletonOnce sync.Once
	)

	return func(e *engine.Engine) sybilprotection.SybilProtection {
		createSingletonOnce.Do(func() {
			singleton = New(e, opts...)
		})

		return singleton
	}
}

func (s *SybilProtection) Init() {
	s.engine.Storage.Permanent.Events.ConsensusWeightsUpdated.Hook(event.NewClosure(s.UpdateConsensusWeights))

	s.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(s.HandleSolidBlock))
	s.engine.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) { s.HandleAcceptedBlock(block.ModelsBlock) }))
	s.engine.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) { s.HandledOrphanedBlock(block.ModelsBlock) }))
	s.engine.Events.EvictionState.EpochEvicted.Hook(event.NewClosure(s.HandleEpochEvicted))
}

func (s *SybilProtection) ActiveValidators() sybilprotection.ActiveValidators {
	return s.activeNodes
}

func (s *SybilProtection) _Attestations(index epoch.Index) (epochAttestations *Attestations) {
	return lo.Return1(s.attestationsByEpoch.Get(index))
}

func (s *SybilProtection) HandleSolidBlock(block *blockdag.Block) {
	s.activeNodes.MarkActive(block.IssuerID(), block.IssuingTime())
}

func (s *SybilProtection) HandleAcceptedBlock(block *models.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	lo.Return1(s.attestationsByEpoch.RetrieveOrCreate(block.ID().Index(), func() *Attestations {
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
	snapshotEpoch := s.engine.Storage.Settings.LatestCommitment().Index()

	for it := s.engine.Storage.Attestors.LoadAll(snapshotEpoch).Iterator(); it.HasNext(); {
		s.activeNodes.MarkActive(it.Next(), snapshotEpoch.EndTime())
	}
}
