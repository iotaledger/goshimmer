package proofofstake

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/weights"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	storageModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

// ProofOfStake is a sybil protection module for the engine that manages the weights of actors according to their stake.
type ProofOfStake struct {
	engine              *engine.Engine
	weights             *weights.Vector
	validators          *weights.Set
	attestationsByEpoch *memstorage.Storage[epoch.Index, *models.Attestations]
	evictionMutex       sync.RWMutex
	inactivityManager   *timed.TaskExecutor[identity.ID]
	lastActivities      *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	mutex               sync.RWMutex
	optsActivityWindow  time.Duration
}

// New creates a new ProofOfStake instance.
func New(engineInstance *engine.Engine, opts ...options.Option[ProofOfStake]) (proofOfStake *ProofOfStake) {
	return options.Apply(
		&ProofOfStake{
			engine:             engineInstance,
			optsActivityWindow: time.Second * 30,
		}, opts, func(p *ProofOfStake) {
			p.weights = weights.NewVector(engineInstance.Storage.ConsensusWeights, engineInstance.Storage.Settings)
			p.validators = p.weights.NewWeightedSet()
			p.attestationsByEpoch = memstorage.New[epoch.Index, *models.Attestations]()
			p.inactivityManager = timed.NewTaskExecutor[identity.ID](1)
			p.lastActivities = shrinkingmap.New[identity.ID, time.Time]()
		})
}

// NewProvider returns a new sybil protection provider that uses the ProofOfStake module.
func NewProvider(opts ...options.Option[ProofOfStake]) engine.ModuleProvider[sybilprotection.SybilProtection] {
	return engine.ProvideModule[sybilprotection.SybilProtection](func(e *engine.Engine) sybilprotection.SybilProtection {
		return New(e, opts...)
	})
}

// Validators returns the set of validators that are currently active.
func (p *ProofOfStake) Validators() *weights.Set {
	return p.validators
}

// Weights returns the current weights that are staked with validators.
func (p *ProofOfStake) Weights() *weights.Vector {
	return p.weights
}

// InitModule initializes the ProofOfStake module after all the dependencies have been injected into the engine.
func (p *ProofOfStake) InitModule() {
	for it := p.engine.Storage.Attestors.LoadAll(p.engine.Storage.Settings.LatestCommitment().Index()).Iterator(); it.HasNext(); {
		p.validators.Add(it.Next())
	}

	p.engine.Storage.Permanent.Events.ConsensusWeightsUpdated.Hook(event.NewClosure(p.UpdateConsensusWeights))
	p.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		p.markValidatorActive(block.IssuerID(), block.IssuingTime())
	}))
	p.engine.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) { p.HandleAcceptedBlock(block.ModelsBlock) }))
	p.engine.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) { p.HandledOrphanedBlock(block.ModelsBlock) }))
	p.engine.Events.EvictionState.EpochEvicted.Hook(event.NewClosure(p.HandleEpochEvicted))
}

func (p *ProofOfStake) Attestations(index epoch.Index) (epochAttestations *models.Attestations) {
	return lo.Return1(p.attestationsByEpoch.Get(index))
}

func (p *ProofOfStake) markValidatorActive(id identity.ID, activityTime time.Time) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if lastActivity, exists := p.lastActivities.Get(id); exists && lastActivity.After(activityTime) {
		return
	} else if !exists {
		p.validators.Add(id)
	}

	p.lastActivities.Set(id, activityTime)

	p.inactivityManager.ExecuteAfter(id, func() { p.markValidatorInactive(id) }, activityTime.Add(p.optsActivityWindow).Sub(p.engine.Clock.RelativeAcceptedTime()))
}

func (p *ProofOfStake) markValidatorInactive(id identity.ID) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.lastActivities.Delete(id)
	p.validators.Delete(id)
}

func (p *ProofOfStake) HandleAcceptedBlock(block *models.Block) {
	p.evictionMutex.RLock()
	defer p.evictionMutex.RUnlock()

	lo.Return1(p.attestationsByEpoch.RetrieveOrCreate(block.ID().Index(), func() *models.Attestations {
		return models.NewAttestations(p.weights)
	})).Add(block)
}

func (p *ProofOfStake) HandledOrphanedBlock(block *models.Block) {
	p.evictionMutex.RLock()
	defer p.evictionMutex.RUnlock()

	if attestations, exists := p.attestationsByEpoch.Get(block.ID().Index()); exists {
		attestations.Delete(block)
	}
}

func (p *ProofOfStake) UpdateConsensusWeights(weightUpdates map[identity.ID]*storageModels.TimedBalance) {
	for id, updateMana := range weightUpdates {
		p.weights.SetWeight(id, updateMana.Balance)
	}
}

func (p *ProofOfStake) Weight(id identity.ID) (weight int64) {
	return p.weights.Weight(id)
}

func (p *ProofOfStake) HandleEpochEvicted(index epoch.Index) {
	p.evictionMutex.Lock()
	defer p.evictionMutex.Unlock()

	p.attestationsByEpoch.Delete(index)
}

// code contract (make sure the type implements all required methods)
var _ sybilprotection.SybilProtection = &ProofOfStake{}
