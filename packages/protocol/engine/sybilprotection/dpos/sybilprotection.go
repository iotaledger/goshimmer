package dpos

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/timed"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	PrefixLastCommittedSlot byte = iota
	PrefixWeights
)

// SybilProtection is a sybil protection module for the engine that manages the weights of actors according to their stake.
type SybilProtection struct {
	engine            *engine.Engine
	workers           *workerpool.Group
	weights           *sybilprotection.Weights
	validators        *sybilprotection.WeightedSet
	inactivityManager *timed.TaskExecutor[identity.ID]
	lastActivities    *shrinkingmap.ShrinkingMap[identity.ID, time.Time]
	weightsBatch      *sybilprotection.WeightsBatch
	mutex             sync.RWMutex

	optsActivityWindow time.Duration

	traits.BatchCommittable
	module.Module
}

// NewSybilProtection creates a new ProofOfStake instance.
func NewSybilProtection(engineInstance *engine.Engine, opts ...options.Option[SybilProtection]) (proofOfStake *SybilProtection) {
	return options.Apply(
		&SybilProtection{
			BatchCommittable: traits.NewBatchCommittable(engineInstance.Storage.SybilProtection(), PrefixLastCommittedSlot),

			engine:            engineInstance,
			workers:           engineInstance.Workers.CreateGroup("SybilProtection"),
			weights:           sybilprotection.NewWeights(engineInstance.Storage.SybilProtection(PrefixWeights)),
			inactivityManager: timed.NewTaskExecutor[identity.ID](1),
			lastActivities:    shrinkingmap.New[identity.ID, time.Time](),

			optsActivityWindow: time.Second * 30,
		}, opts, func(s *SybilProtection) {
			s.validators = s.weights.NewWeightedSet()

			s.engine.HookConstructed(func() {
				s.engine.Ledger.HookInitialized(s.initializeTotalWeight)
				s.engine.Ledger.UnspentOutputs().HookInitialized(s.initializeLatestCommitment)
				s.engine.Notarization.Attestations().HookInitialized(s.initializeActiveValidators)

				s.engine.Ledger.UnspentOutputs().Subscribe(s)

				s.engine.HookStopped(s.stopInactivityManager)

				s.engine.Events.Tangle.BlockDAG.BlockSolid.Hook(func(block *blockdag.Block) {
					s.markValidatorActive(block.IssuerID(), block.IssuingTime())
				}, event.WithWorkerPool(s.workers.CreatePool("SybilProtection", 2)))
			})
		})
}

func (s *SybilProtection) initializeLatestCommitment() {
	s.SetLastCommittedSlot(s.engine.Storage.Settings.LatestCommitment().Index())
}

func (s *SybilProtection) initializeTotalWeight() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.weights.UpdateTotalWeightSlot(s.engine.Storage.Settings.LatestCommitment().Index())
}

func (s *SybilProtection) initializeActiveValidators() {
	attestations, err := s.engine.Notarization.Attestations().Get(s.engine.Storage.Settings.LatestCommitment().Index())
	if err != nil {
		panic(err)
	}

	if err = attestations.Stream(func(id identity.ID, attestation *notarization.Attestation) bool {
		s.validators.Add(id)

		return true
	}); err != nil {
		panic(err)
	}

	s.TriggerInitialized()
}

func (s *SybilProtection) stopInactivityManager() {
	s.inactivityManager.Shutdown(timed.CancelPendingElements)
}

// NewProvider returns a new sybil protection provider that uses the ProofOfStake module.
func NewProvider(opts ...options.Option[SybilProtection]) module.Provider[*engine.Engine, sybilprotection.SybilProtection] {
	return module.Provide(func(e *engine.Engine) sybilprotection.SybilProtection {
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

func (s *SybilProtection) ApplyCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		if s.BatchedStateTransitionStarted() {
			s.weightsBatch.Update(output.ConsensusManaPledgeID(), int64(iotaBalance))
		} else {
			s.weights.Update(output.ConsensusManaPledgeID(), sybilprotection.NewWeight(int64(iotaBalance), output.Index()))
		}
	}

	return
}

func (s *SybilProtection) ApplySpentOutput(output *mempool.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		if s.BatchedStateTransitionStarted() {
			s.weightsBatch.Update(output.ConsensusManaPledgeID(), -int64(iotaBalance))
		} else {
			s.weights.Update(output.ConsensusManaPledgeID(), sybilprotection.NewWeight(-int64(iotaBalance), output.Index()))
		}
	}

	return
}

func (s *SybilProtection) RollbackCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	return s.ApplySpentOutput(output)
}

func (s *SybilProtection) RollbackSpentOutput(output *mempool.OutputWithMetadata) (err error) {
	return s.ApplyCreatedOutput(output)
}

func (s *SybilProtection) BeginBatchedStateTransition(newSlot slot.Index) (currentSlot slot.Index, err error) {
	if currentSlot, err = s.BatchCommittable.BeginBatchedStateTransition(newSlot); err != nil {
		return 0, errors.Wrap(err, "failed to begin batched state transition")
	}

	if currentSlot != newSlot {
		s.weightsBatch = sybilprotection.NewWeightsBatch(newSlot)
	}

	return
}

func (s *SybilProtection) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		s.weights.BatchUpdate(s.weightsBatch)

		s.FinalizeBatchedStateTransition()

		done()
	}()

	return ctx
}

func (s *SybilProtection) markValidatorActive(id identity.ID, activityTime time.Time) {
	if s.engine.WasStopped() {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if lastActivity, exists := s.lastActivities.Get(id); exists && lastActivity.After(activityTime) {
		return
	} else if !exists {
		s.validators.Add(id)
	}

	s.lastActivities.Set(id, activityTime)

	s.inactivityManager.ExecuteAfter(id, func() { s.markValidatorInactive(id) }, activityTime.Add(s.optsActivityWindow).Sub(s.engine.Clock.Accepted().RelativeTime()))
}

func (s *SybilProtection) markValidatorInactive(id identity.ID) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastActivities.Delete(id)
	s.validators.Delete(id)
}

var _ ledger.UnspentOutputsSubscriber = &SybilProtection{}
