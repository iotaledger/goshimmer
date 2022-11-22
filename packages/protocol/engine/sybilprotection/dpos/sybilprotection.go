package dpos

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/state"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	ledgerModels "github.com/iotaledger/goshimmer/packages/storage/models"
)

// SybilProtection is a sybil protection module for the engine that manages the weights of actors according to their stake.
type SybilProtection struct {
	engine             *engine.Engine
	weights            *sybilprotection.Weights
	validators         *sybilprotection.WeightedSet
	inactivityManager  *timed.TaskExecutor[identity.ID]
	lastActivities     *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	mutex              sync.RWMutex
	optsActivityWindow time.Duration
}

// NewSybilProtection creates a new ProofOfStake instance.
func NewSybilProtection(engineInstance *engine.Engine, opts ...options.Option[SybilProtection]) (proofOfStake *SybilProtection) {
	return options.Apply(
		&SybilProtection{
			engine:             engineInstance,
			optsActivityWindow: time.Second * 30,
		}, opts, func(p *SybilProtection) {
			p.weights = sybilprotection.NewWeights(engineInstance.Storage.SybilProtection, engineInstance.Storage.Settings)
			p.validators = p.weights.WeightedSet()
			p.inactivityManager = timed.NewTaskExecutor[identity.ID](1)
			p.lastActivities = shrinkingmap.New[identity.ID, time.Time]()
		})
}

// NewSybilProtectionProvider returns a new sybil protection provider that uses the ProofOfStake module.
func NewSybilProtectionProvider(opts ...options.Option[SybilProtection]) engine.ModuleProvider[sybilprotection.SybilProtection] {
	return engine.ProvideModule[sybilprotection.SybilProtection](func(e *engine.Engine) sybilprotection.SybilProtection {
		return NewSybilProtection(e, opts...)
	})
}

// Validators returns the set of validators that are currently active.
func (p *SybilProtection) Validators() *sybilprotection.WeightedSet {
	return p.validators
}

// Weights returns the current weights that are staked with validators.
func (p *SybilProtection) Weights() *sybilprotection.Weights {
	return p.weights
}

func (p *SybilProtection) ImportOutput(output *ledgerModels.OutputWithMetadata) {
	ProcessCreatedOutput(output, p.weights.Import)
}

func (p *SybilProtection) BatchedTransition(targetEpoch epoch.Index) state.BatchedTransition {
	return NewBatchedTransition(p.weights, targetEpoch)
}

func (p *SybilProtection) LastConsumedEpoch() epoch.Index {
	return p.weights.TotalWeight().UpdateTime
}

// InitModule initializes the ProofOfStake module after all the dependencies have been injected into the engine.
func (p *SybilProtection) InitModule() {
	for it := p.engine.Storage.Attestors.LoadAll(p.engine.Storage.Settings.LatestCommitment().Index()).Iterator(); it.HasNext(); {
		p.validators.Add(it.Next())
	}

	p.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		p.markValidatorActive(block.IssuerID(), block.IssuingTime())
	}))
}

func (p *SybilProtection) markValidatorActive(id identity.ID, activityTime time.Time) {
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

func (p *SybilProtection) markValidatorInactive(id identity.ID) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.lastActivities.Delete(id)
	p.validators.Delete(id)
}

// code contract (make sure the type implements all required methods)
var _ sybilprotection.SybilProtection = &SybilProtection{}
