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
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
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

	lastCommittedEpoch      epoch.Index
	lastCommittedEpochMutex sync.RWMutex
	batchedEpochIndex       epoch.Index
	batchedEpochMutex       sync.RWMutex
	batchedWeightUpdates    *sybilprotection.WeightUpdates
}

// NewSybilProtection creates a new ProofOfStake instance.
func NewSybilProtection(engineInstance *engine.Engine, opts ...options.Option[SybilProtection]) (proofOfStake *SybilProtection) {
	return options.Apply(
		&SybilProtection{
			engine:             engineInstance,
			optsActivityWindow: time.Second * 30,
		}, opts, func(s *SybilProtection) {
			s.weights = sybilprotection.NewWeights(engineInstance.Storage.SybilProtection, engineInstance.Storage.Settings)
			s.validators = s.weights.WeightedSet()
			s.inactivityManager = timed.NewTaskExecutor[identity.ID](1)
			s.lastActivities = shrinkingmap.New[identity.ID, time.Time]()

			s.engine.LedgerState.RegisterConsumer(s)
		})
}

// NewSybilProtectionProvider returns a new sybil protection provider that uses the ProofOfStake module.
func NewSybilProtectionProvider(opts ...options.Option[SybilProtection]) engine.ModuleProvider[sybilprotection.SybilProtection] {
	return engine.ProvideModule(func(e *engine.Engine) sybilprotection.SybilProtection {
		return NewSybilProtection(e, opts...)
	})
}

// Validators returns the set of validators that are currently active.
func (s *SybilProtection) Validators() *sybilprotection.WeightedSet {
	return s.validators
}

// Weights returns the current weights that are staked with validators.
func (s *SybilProtection) Weights() *sybilprotection.Weights {
	return s.weights
}

// InitModule initializes the ProofOfStake module after all the dependencies have been injected into the engine.
func (s *SybilProtection) InitModule() {
	for it := s.engine.Storage.Attestors.LoadAll(s.engine.Storage.Settings.LatestCommitment().Index()).Iterator(); it.HasNext(); {
		s.validators.Add(it.Next())
	}

	s.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		s.markValidatorActive(block.IssuerID(), block.IssuingTime())
	}))
}

func (s *SybilProtection) markValidatorActive(id identity.ID, activityTime time.Time) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if lastActivity, exists := s.lastActivities.Get(id); exists && lastActivity.After(activityTime) {
		return
	} else if !exists {
		s.validators.Add(id)
	}

	s.lastActivities.Set(id, activityTime)

	s.inactivityManager.ExecuteAfter(id, func() { s.markValidatorInactive(id) }, activityTime.Add(s.optsActivityWindow).Sub(s.engine.Clock.RelativeAcceptedTime()))
}

func (s *SybilProtection) markValidatorInactive(id identity.ID) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastActivities.Delete(id)
	s.validators.Delete(id)
}
