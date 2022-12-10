package dpos

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/timed"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
)

const (
	PrefixLastCommittedEpoch byte = iota
	PrefixWeights
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

	batchedWeightUpdates *sybilprotection.WeightUpdatesBatch

	traits.Initializable
	traits.BatchCommittable
}

// NewSybilProtection creates a new ProofOfStake instance.
func NewSybilProtection(engineInstance *engine.Engine, opts ...options.Option[SybilProtection]) (proofOfStake *SybilProtection) {
	return options.Apply(
		&SybilProtection{
			Initializable:    traits.NewInitializable(),
			BatchCommittable: traits.NewBatchCommittable(engineInstance.Storage.SybilProtection(PrefixLastCommittedEpoch)),

			engine:            engineInstance,
			weights:           sybilprotection.NewWeights(engineInstance.Storage.SybilProtection(PrefixWeights), engineInstance.Storage.Settings),
			inactivityManager: timed.NewTaskExecutor[identity.ID](1),
			lastActivities:    shrinkingmap.New[identity.ID, time.Time](),

			optsActivityWindow: time.Second * 30,
		}, opts, func(s *SybilProtection) {
			s.validators = s.weights.WeightedSet()

			s.engine.SubscribeConstructed(func() {
				traits.SubscribeInitialized(map[traits.Initializable]func(){
					s.engine.LedgerState:                      s.initializeTotalWeight,
					s.engine.LedgerState.UnspentOutputs:       s.initializeLatestCommitment,
					s.engine.NotarizationManager.Attestations: s.initializeActiveValidators,
				})

				s.engine.LedgerState.UnspentOutputs.Subscribe(s)

				s.engine.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
					s.markValidatorActive(block.IssuerID(), block.IssuingTime())
				}))
			})
		})
}

func (s *SybilProtection) initializeLatestCommitment() {
	s.SetLastCommittedEpoch(s.engine.Storage.Settings.LatestCommitment().Index())
}

func (s *SybilProtection) initializeTotalWeight() {
	s.weights.TotalWeight().UpdateTime = s.engine.Storage.Settings.LatestCommitment().Index()
}

func (s *SybilProtection) initializeActiveValidators() {
	attestations, err := s.engine.NotarizationManager.Attestations.Get(s.engine.Storage.Settings.LatestCommitment().Index())
	if err != nil {
		panic(err)
	}

	if err = attestations.Stream(func(id identity.ID, attestation *notarization.Attestation) bool {
		s.validators.Add(id)

		return true
	}); err != nil {
		panic(err)
	}
}

// NewProvider returns a new sybil protection provider that uses the ProofOfStake module.
func NewProvider(opts ...options.Option[SybilProtection]) engine.ModuleProvider[sybilprotection.SybilProtection] {
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

func (s *SybilProtection) ApplyCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if !s.BatchedStateTransitionStarted() {
		ApplyCreatedOutput(output, s.weights.ImportDiff)
	} else {
		ApplyCreatedOutput(output, s.batchedWeightUpdates.ApplyDiff)
	}

	return
}

func (s *SybilProtection) ApplySpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if !s.BatchedStateTransitionStarted() {
		ApplySpentOutput(output, s.weights.ImportDiff)
	} else {
		ApplySpentOutput(output, s.batchedWeightUpdates.ApplyDiff)
	}

	return
}

func (s *SybilProtection) RollbackCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return s.ApplySpentOutput(output)
}

func (s *SybilProtection) RollbackSpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return s.ApplyCreatedOutput(output)
}

func (s *SybilProtection) BeginBatchedStateTransition(newEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	if currentEpoch, err = s.BatchCommittable.BeginBatchedStateTransition(newEpoch); err != nil {
		return 0, errors.Errorf("failed to begin batched state transition: %w", err)
	}

	if currentEpoch != newEpoch {
		s.batchedWeightUpdates = sybilprotection.NewWeightUpdates(newEpoch)
	}

	return
}

func (s *SybilProtection) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())
	go func() {
		s.weights.ApplyBatchUpdates(s.batchedWeightUpdates)

		s.FinalizeBatchedStateTransition()

		done()
	}()

	return ctx
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

var _ ledgerstate.UnspentOutputsConsumer = &SybilProtection{}
