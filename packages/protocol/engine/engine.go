package engine

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/storage"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Engine /////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events              *Events
	Storage             *storage.Storage
	SybilProtection     sybilprotection.SybilProtection
	ThroughputQuota     throughputquota.ThroughputQuota
	Ledger              *ledger.Ledger
	LedgerState         *ledgerstate.LedgerState
	Filter              *filter.Filter
	EvictionState       *eviction.State
	BlockRequester      *eventticker.EventTicker[models.BlockID]
	NotarizationManager *notarization.Manager
	Tangle              *tangle.Tangle
	Consensus           *consensus.Consensus
	TSCManager          *tsc.Manager
	Clock               *clock.Clock

	workerPools map[string]*workerpool.UnboundedWorkerPool

	isBootstrapped      bool
	isBootstrappedMutex sync.Mutex

	optsBootstrappedThreshold      time.Duration
	optsEntryPointsDepth           int
	optsSnapshotDepth              int
	optsLedgerOptions              []options.Option[ledger.Ledger]
	optsNotarizationManagerOptions []options.Option[notarization.Manager]
	optsTangleOptions              []options.Option[tangle.Tangle]
	optsConsensusOptions           []options.Option[consensus.Consensus]
	optsTSCManagerOptions          []options.Option[tsc.Manager]
	optsBlockRequester             []options.Option[eventticker.EventTicker[models.BlockID]]

	traits.Constructable
	traits.Initializable
	traits.Stoppable
}

func New(
	storageInstance *storage.Storage,
	sybilProtection ModuleProvider[sybilprotection.SybilProtection],
	throughputQuota ModuleProvider[throughputquota.ThroughputQuota],
	opts ...options.Option[Engine],
) (engine *Engine) {
	return options.Apply(
		&Engine{
			Events:        NewEvents(),
			Storage:       storageInstance,
			Constructable: traits.NewConstructable(),
			Stoppable:     traits.NewStoppable(),

			workerPools: map[string]*workerpool.UnboundedWorkerPool{},

			optsBootstrappedThreshold: 10 * time.Second,
			optsSnapshotDepth:         5,
		}, opts, func(e *Engine) {
			e.Ledger = ledger.New(e.Storage, e.optsLedgerOptions...)
			e.LedgerState = ledgerstate.New(storageInstance, e.Ledger)
			e.Clock = clock.New()
			e.EvictionState = eviction.NewState(storageInstance)
			e.SybilProtection = sybilProtection(e)
			e.ThroughputQuota = throughputQuota(e)
			e.NotarizationManager = notarization.NewManager(e.Storage, e.LedgerState, e.SybilProtection.Weights(), e.optsNotarizationManagerOptions...)

			e.Initializable = traits.NewInitializable(
				e.Storage.Settings.TriggerInitialized,
				e.Storage.Commitments.TriggerInitialized,
				e.LedgerState.TriggerInitialized,
				e.NotarizationManager.TriggerInitialized,
			)
		},

		(*Engine).initFilter,
		(*Engine).initLedger,
		(*Engine).initTangle,
		(*Engine).initConsensus,
		(*Engine).initClock,
		(*Engine).initTSCManager,
		(*Engine).initBlockStorage,
		(*Engine).initNotarizationManager,
		(*Engine).initEvictionState,
		(*Engine).initBlockRequester,

		func(e *Engine) {
			e.TriggerConstructed()
		},
	)
}

func (e *Engine) WorkerPools() map[string]*workerpool.UnboundedWorkerPool {
	return e.workerPools
}

func (e *Engine) ProcessBlockFromPeer(block *models.Block, source identity.ID) {
	e.Filter.ProcessReceivedBlock(block, source)
}

func (e *Engine) Block(id models.BlockID) (block *models.Block, exists bool) {
	var err error
	if e.EvictionState.IsRootBlock(id) {
		block, err = e.Storage.Blocks.Load(id)
		exists = block != nil && err == nil
		return
	}

	if cachedBlock, cachedBlockExists := e.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.ModelsBlock, !cachedBlock.IsMissing()
	}

	if id.Index() > e.Storage.Settings.LatestCommitment().Index() {
		return nil, false
	}

	block, err = e.Storage.Blocks.Load(id)
	exists = block != nil && err == nil

	return
}

func (e *Engine) FirstUnacceptedMarker(sequenceID markers.SequenceID) markers.Index {
	if e.Consensus == nil {
		return 1
	}

	return e.Consensus.BlockGadget.FirstUnacceptedIndex(sequenceID)
}

func (e *Engine) LastConfirmedEpoch() epoch.Index {
	if e.Consensus == nil {
		return 0
	}

	return e.Consensus.EpochGadget.LastConfirmedEpoch()
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	e.isBootstrappedMutex.Lock()
	defer e.isBootstrappedMutex.Unlock()

	if e.isBootstrapped {
		return true
	}

	if isBootstrapped = time.Since(e.Clock.RelativeAcceptedTime()) < e.optsBootstrappedThreshold && e.NotarizationManager.IsFullyCommitted(); isBootstrapped {
		e.isBootstrapped = true
	}

	return isBootstrapped
}

func (e *Engine) IsSynced() (isBootstrapped bool) {
	return e.IsBootstrapped() && time.Since(e.Clock.AcceptedTime()) < e.optsBootstrappedThreshold
}

func (e *Engine) Initialize(snapshot string) (err error) {
	if !e.Storage.Settings.SnapshotImported() {
		if err = e.readSnapshot(snapshot); err != nil {
			return errors.Errorf("failed to read snapshot from file '%s': %w", snapshot, err)
		}
	}

	e.TriggerInitialized()

	return
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()

	e.TriggerStopped()
}

func (e *Engine) WriteSnapshot(filePath string, targetEpoch ...epoch.Index) (err error) {
	if len(targetEpoch) == 0 {
		targetEpoch = append(targetEpoch, e.Storage.Settings.LatestCommitment().Index())
	}

	if fileHandle, err := os.Create(filePath); err != nil {
		return errors.Errorf("failed to create snapshot file: %w", err)
	} else if err = e.Export(fileHandle, targetEpoch[0]); err != nil {
		return errors.Errorf("failed to write snapshot: %w", err)
	} else if err = fileHandle.Close(); err != nil {
		return errors.Errorf("failed to close snapshot file: %w", err)
	}

	return
}

func (e *Engine) Import(reader io.ReadSeeker) (err error) {
	if err = e.Storage.Settings.Import(reader); err != nil {
		return errors.Errorf("failed to import settings: %w", err)
	} else if err = e.Storage.Commitments.Import(reader); err != nil {
		return errors.Errorf("failed to import commitments: %w", err)
	} else if err = e.Storage.Settings.SetChainID(e.Storage.Settings.LatestCommitment().ID()); err != nil {
		return errors.Errorf("failed to set chainID: %w", err)
	} else if err = e.EvictionState.Import(reader); err != nil {
		return errors.Errorf("failed to import eviction state: %w", err)
	} else if err = e.LedgerState.Import(reader); err != nil {
		return errors.Errorf("failed to import ledger state: %w", err)
	} else if err = e.NotarizationManager.Import(reader); err != nil {
		return errors.Errorf("failed to import notarization state: %w", err)
	}

	// We need to set the genesis time before we add the activity log as otherwise the calculation is based on the empty time value.
	e.Clock.SetAcceptedTime(e.Storage.Settings.LatestCommitment().Index().EndTime())
	e.Clock.SetConfirmedTime(e.Storage.Settings.LatestCommitment().Index().EndTime())

	return
}

func (e *Engine) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = e.Storage.Settings.Export(writer); err != nil {
		return errors.Errorf("failed to export settings: %w", err)
	} else if err = e.Storage.Commitments.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export commitments: %w", err)
	} else if err = e.EvictionState.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export eviction state: %w", err)
	} else if err = e.LedgerState.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export ledger state: %w", err)
	} else if err = e.NotarizationManager.Export(writer, targetEpoch); err != nil {
		return errors.Errorf("failed to export notarization state: %w", err)
	}

	return
}

func (e *Engine) initFilter() {
	e.Filter = filter.New(e.Tangle.BlockDAG.Block)

	e.Events.Filter.LinkTo(e.Filter.Events)
}

func (e *Engine) initLedger() {
	e.Events.Ledger.LinkTo(e.Ledger.Events)
}

func (e *Engine) initLedgerState() {
}

func (e *Engine) initTangle() {
	e.Tangle = tangle.New(e.Ledger, e.EvictionState, e.SybilProtection.Validators(), e.LastConfirmedEpoch, e.FirstUnacceptedMarker, e.optsTangleOptions...)

	e.Events.Filter.BlockAllowed.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := e.Tangle.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to attach block with %s (issuerID: %s): %w", block.ID(), block.IssuerID(), err))
		}
	}))

	e.Events.Tangle.LinkTo(e.Tangle.Events)
}

func (e *Engine) initConsensus() {
	e.Consensus = consensus.New(e.Tangle, e.EvictionState, e.Storage.Permanent.Settings.LatestConfirmedEpoch(), func() (totalWeight int64) {
		if zeroIdentityWeight, exists := e.SybilProtection.Weights().Get(identity.ID{}); exists {
			totalWeight -= zeroIdentityWeight.Value
		}

		return totalWeight + e.SybilProtection.Weights().TotalWeight().Value
	}, e.optsConsensusOptions...)

	e.Events.EvictionState.EpochEvicted.Hook(event.NewClosure(e.Consensus.BlockGadget.EvictUntil))

	e.Events.Consensus.LinkTo(e.Consensus.Events)

	e.Events.Consensus.BlockGadget.Error.Hook(event.NewClosure(func(err error) {
		e.Events.Error.Trigger(err)
	}))

	e.Events.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		err := e.Storage.Permanent.Settings.SetLatestConfirmedEpoch(epochIndex)
		if err != nil {
			panic(err)
		}

		e.Tangle.VirtualVoting.EvictEpochTracker(epochIndex)
	}))
}

func (e *Engine) initClock() {
	e.workerPools["Clock.SetAcceptedTime"] = e.Events.Consensus.BlockGadget.BlockAccepted.AttachWithNewWorkerPool(event.NewClosure(func(block *blockgadget.Block) {
		e.Clock.SetAcceptedTime(block.IssuingTime())
	}), 1)

	e.workerPools["Clock.SetConfirmedTime"] = e.Events.Consensus.BlockGadget.BlockConfirmed.AttachWithNewWorkerPool(event.NewClosure(func(block *blockgadget.Block) {
		e.Clock.SetConfirmedTime(block.IssuingTime())
	}), 1)

	e.Events.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		e.Clock.SetConfirmedTime(epochIndex.EndTime())
	}))

	e.Events.Clock.LinkTo(e.Clock.Events)
}

func (e *Engine) initTSCManager() {
	e.TSCManager = tsc.New(e.Consensus.BlockGadget.IsBlockAccepted, e.Tangle, e.optsTSCManagerOptions...)

	e.Events.Tangle.Booker.BlockBooked.Attach(event.NewClosure(e.TSCManager.AddBlock))

	e.Events.Clock.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *clock.TimeUpdateEvent) {
		e.TSCManager.HandleTimeUpdate(event.NewTime)
	}))
}

func (e *Engine) initBlockStorage() {
	e.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		if err := e.Storage.Blocks.Store(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to store block with %s: %w", block.ID(), err))
		}
	}))

	e.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		if err := e.Storage.Blocks.Delete(block.ID()); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to delete block with %s: %w", block.ID(), err))
		}
	}))
}

func (e *Engine) initNotarizationManager() {
	wp := e.Consensus.BlockGadget.Events.BlockAccepted.AttachWithNewWorkerPool(event.NewClosure(func(block *blockgadget.Block) {
		if err := e.NotarizationManager.NotarizeAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted block %s to epoch: %w", block.ID(), err))
		}
	}), 1)
	e.Tangle.Events.BlockDAG.BlockOrphaned.AttachWithWorkerPool(event.NewClosure(func(block *blockdag.Block) {
		if err := e.NotarizationManager.NotarizeOrphanedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to remove orphaned block %s from epoch: %w", block.ID(), err))
		}
	}), wp)
	e.workerPools["NotarizationManager.Blocks"] = wp

	// TODO: Why is it hooked?
	e.Ledger.Events.TransactionAccepted.Hook(event.NewClosure(func(event *ledger.TransactionEvent) {
		if err := e.NotarizationManager.EpochMutations.AddAcceptedTransaction(event.Metadata); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted transaction %s to epoch: %w", event.Metadata.ID(), err))
		}
	}))
	e.Ledger.Events.TransactionInclusionUpdated.Hook(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		if err := e.NotarizationManager.EpochMutations.UpdateTransactionInclusion(event.TransactionID, event.PreviousInclusionEpoch, event.InclusionEpoch); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to update transaction inclusion time %s in epoch: %w", event.TransactionID, err))
		}
	}))
	// TODO: add transaction orphaned event

	// Epochs are committed whenever ATT advances, start committing only when bootstrapped.
	wp = e.Clock.Events.AcceptanceTimeUpdated.AttachWithNewWorkerPool(event.NewClosure(func(event *clock.TimeUpdateEvent) {
		e.NotarizationManager.SetAcceptanceTime(event.NewTime)
	}), 1)

	e.Ledger.ConflictDAG.Events.ConflictCreated.AttachWithWorkerPool(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		e.NotarizationManager.IncreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(event.ID).IssuingTime()))
	}), wp)
	e.Ledger.ConflictDAG.Events.ConflictAccepted.AttachWithWorkerPool(event.NewClosure(func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}), wp)
	e.Ledger.ConflictDAG.Events.ConflictRejected.AttachWithWorkerPool(event.NewClosure(func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}), wp)
	e.workerPools["NotarizationManager.Commitments"] = wp

	e.Events.NotarizationManager.LinkTo(e.NotarizationManager.Events)
	e.Events.EpochMutations.LinkTo(e.NotarizationManager.EpochMutations.Events)
}

func (e *Engine) initEvictionState() {
	e.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		block.ForEachParent(func(parent models.Parent) {
			// TODO: ONLY ADD STRONG PARENTS AFTER NOT DOWNLOADING PAST WEAK ARROWS
			// TODO: is this correct? could this lock acceptance in some extreme corner case? something like this happened, that confirmation is correctly advancing per block, but acceptance does not. I think it might have something to do with root blocks
			if parent.ID.Index() < block.ID().Index() {
				e.EvictionState.AddRootBlock(parent.ID)
			}
		})
	}))

	e.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.EvictionState.RemoveRootBlock(block.ID())
	}))

	e.NotarizationManager.Events.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
		e.EvictionState.EvictUntil(details.Commitment.Index())
	}))

	e.LedgerState.SubscribeInitialized(func() {
		e.EvictionState.EvictUntil(e.Storage.Settings.LatestCommitment().Index())
	})

	e.Events.EvictionState.LinkTo(e.EvictionState.Events)
}

func (e *Engine) initBlockRequester() {
	e.BlockRequester = eventticker.New(e.optsBlockRequester...)

	e.Events.EvictionState.EpochEvicted.Hook(event.NewClosure(e.BlockRequester.EvictUntil))

	// We need to hook to make sure that the request is created before the block arrives to avoid a race condition
	// where we try to delete the request again before it is created. Thus, continuing to request forever.
	e.Events.Tangle.BlockDAG.BlockMissing.Hook(event.NewClosure(func(block *blockdag.Block) {
		// TODO: ONLY START REQUESTING WHEN NOT IN WARPSYNC RANGE (or just not attach outside)?
		e.BlockRequester.StartTicker(block.ID())
	}))
	e.Events.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.BlockRequester.StopTicker(block.ID())
	}))

	e.Events.BlockRequester.LinkTo(e.BlockRequester.Events)
}

func (e *Engine) readSnapshot(filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Errorf("failed to open snapshot file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	if err = e.Import(file); err != nil {
		return errors.Errorf("failed to import snapshot: %w", err)
	} else if err = e.Storage.Settings.SetSnapshotImported(true); err != nil {
		return errors.Errorf("failed to set snapshot imported flag: %w", err)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBootstrapThreshold(threshold time.Duration) options.Option[Engine] {
	return func(e *Engine) {
		e.optsBootstrappedThreshold = threshold
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTangleOptions = opts
	}
}

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensusOptions = opts
	}
}

func WithEntryPointsDepth(entryPointsDepth int) options.Option[Engine] {
	return func(engine *Engine) {
		engine.optsEntryPointsDepth = entryPointsDepth
	}
}

func WithTSCManagerOptions(opts ...options.Option[tsc.Manager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTSCManagerOptions = opts
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsLedgerOptions = opts
	}
}

func WithNotarizationManagerOptions(opts ...options.Option[notarization.Manager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsNotarizationManagerOptions = opts
	}
}

func WithSnapshotDepth(depth int) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSnapshotDepth = depth
	}
}

func WithRequesterOptions(opts ...options.Option[eventticker.EventTicker[models.BlockID]]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsBlockRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
