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

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/initializable"
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

	isBootstrapped      bool
	isBootstrappedMutex sync.Mutex

	optsSybilProtectionProvider    func(engine *Engine) sybilprotection.SybilProtection
	optsThroughputQuotaProvider    func(engine *Engine) throughputquota.ThroughputQuota
	optsBootstrappedThreshold      time.Duration
	optsEntryPointsDepth           int
	optsSnapshotDepth              int
	optsLedgerOptions              []options.Option[ledger.Ledger]
	optsNotarizationManagerOptions []options.Option[notarization.Manager]
	optsTangleOptions              []options.Option[tangle.Tangle]
	optsConsensusOptions           []options.Option[consensus.Consensus]
	optsTSCManagerOptions          []options.Option[tsc.Manager]
	optsBlockRequester             []options.Option[eventticker.EventTicker[models.BlockID]]

	*initializable.Initializable
}

func New(storageInstance *storage.Storage, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(
		&Engine{
			Events:        NewEvents(),
			Storage:       storageInstance,
			LedgerState:   ledgerstate.New(storageInstance),
			Clock:         clock.New(),
			EvictionState: eviction.NewState(storageInstance),

			optsSybilProtectionProvider: panicOnEmptyProvider[sybilprotection.SybilProtection],
			optsThroughputQuotaProvider: panicOnEmptyProvider[throughputquota.ThroughputQuota],
			optsBootstrappedThreshold:   10 * time.Second,
			optsSnapshotDepth:           5,
		}, opts,
		(*Engine).injectPlugins,

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

		(*Engine).initPlugins,
	)
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

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
}

func (e *Engine) LastConfirmedEpoch() epoch.Index {
	if e.Consensus == nil {
		return 0
	}

	return e.Consensus.EpochGadget.LastConfirmedEpoch()
}

func (e *Engine) FirstUnacceptedMarker(sequenceID markers.SequenceID) markers.Index {
	if e.Consensus == nil {
		return 1
	}

	return e.Consensus.BlockGadget.FirstUnacceptedIndex(sequenceID)
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
	} else if err = e.Storage.Attestors.Import(reader); err != nil {
		return errors.Errorf("failed to import attestors: %w", err)
	} else if err = e.LedgerState.Import(reader); err != nil {
		return errors.Errorf("failed to import ledger state: %w", err)
	}

	// We need to set the genesis time before we add the activity log as otherwise the calculation is based on the empty time value.
	e.Clock.SetAcceptedTime(e.Storage.Settings.LatestCommitment().Index().EndTime())
	e.Clock.SetConfirmedTime(e.Storage.Settings.LatestCommitment().Index().EndTime())

	// Ledgerstate
	// TODO: FIX
	// {
	// 	ProcessChunks(NewChunkedReader[ledgerstate.OutputWithMetadata](fileHandle),
	// 		e.ManaTracker.ImportOutputs,
	// 		// This will import into all the consumers too: sybilprotection and ledgerState.unspentOutputIDs
	// 	)
	// }

	return
}

func (e *Engine) Export(writer io.WriteSeeker, epoch epoch.Index) (err error) {
	if err = e.Storage.Settings.Export(writer); err != nil {
		return errors.Errorf("failed to export settings: %w", err)
	} else if err = e.Storage.Commitments.Export(writer, epoch); err != nil {
		return errors.Errorf("failed to export commitments: %w", err)
	} else if err = e.EvictionState.Export(writer, epoch); err != nil {
		return errors.Errorf("failed to export eviction state: %w", err)
	} else if err = e.Storage.Attestors.Export(writer, epoch-1); err != nil {
		return errors.Errorf("failed to export attestors: %w", err)
	} else if err = e.LedgerState.Export(writer, epoch); err != nil {
		return errors.Errorf("failed to export ledger state: %w", err)
	}

	return
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

func (e *Engine) injectPlugins() {
	e.SybilProtection = e.optsSybilProtectionProvider(e)
	e.ThroughputQuota = e.optsThroughputQuotaProvider(e)

	e.Initializable = initializable.New(
		e.Storage.Settings.TriggerInitialized,
		e.Storage.Commitments.TriggerInitialized,
		e.LedgerState.TriggerInitialized,
	)
}

func (e *Engine) initPlugins() {
	e.SybilProtection.Init()
	e.ThroughputQuota.Init()
}

func (e *Engine) initFilter() {
	e.Filter = filter.New()

	e.Events.Filter.LinkTo(e.Filter.Events)
}

func (e *Engine) initLedger() {
	e.Ledger = ledger.New(e.Storage, e.optsLedgerOptions...)

	e.Events.Ledger.LinkTo(e.Ledger.Events)
}

func (e *Engine) initTangle() {
	e.Tangle = tangle.New(e.Ledger, e.EvictionState, e.SybilProtection.Validators(), e.LastConfirmedEpoch, e.FirstUnacceptedMarker, e.optsTangleOptions...)

	e.Events.Filter.BlockReceived.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := e.Tangle.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to attach block with %s (issuerID: %s): %w", block.ID(), block.IssuerID(), err))
		}
	}))

	e.Events.Tangle.LinkTo(e.Tangle.Events)
}

func (e *Engine) initConsensus() {
	e.Consensus = consensus.New(e.Tangle, e.EvictionState, e.Storage.Permanent.Settings.LatestConfirmedEpoch(), e.SybilProtection.Validators().TotalWeight, e.optsConsensusOptions...)

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
	e.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		e.Clock.SetAcceptedTime(block.IssuingTime())
	}))

	e.Events.Consensus.BlockGadget.BlockConfirmed.Attach(event.NewClosure(func(block *blockgadget.Block) {
		e.Clock.SetConfirmedTime(block.IssuingTime())
	}))

	e.Events.Consensus.EpochGadget.EpochConfirmed.Attach(event.NewClosure(func(epochIndex epoch.Index) {
		e.Clock.SetConfirmedTime(epochIndex.EndTime())
	}))

	e.Events.Clock.LinkTo(e.Clock.Events)
}

func (e *Engine) initTSCManager() {
	e.TSCManager = tsc.New(e.Consensus.BlockGadget.IsBlockAccepted, e.Tangle, e.optsTSCManagerOptions...)

	e.Events.Tangle.Booker.BlockBooked.Attach(event.NewClosure(e.TSCManager.AddBlock))

	e.Events.Clock.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *clock.TimeUpdate) {
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
	e.NotarizationManager = notarization.NewManager(e.Storage, e.LedgerState, e.SybilProtection.Weights(), e.optsNotarizationManagerOptions...)

	e.Consensus.BlockGadget.Events.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		if err := e.NotarizationManager.AddAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted block %s to epoch: %w", block.ID(), err))
		}
	}))
	e.Tangle.Events.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		if err := e.NotarizationManager.RemoveAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to remove orphaned block %s from epoch: %w", block.ID(), err))
		}
	}))

	e.Ledger.Events.TransactionAccepted.Attach(event.NewClosure(func(txMeta *ledger.TransactionMetadata) {
		if err := e.NotarizationManager.AddAcceptedTransaction(txMeta); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted transaction %s to epoch: %w", txMeta.ID(), err))
		}
	}))
	e.Ledger.Events.TransactionInclusionUpdated.Attach(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		if err := e.NotarizationManager.UpdateTransactionInclusion(event.TransactionID, epoch.IndexFromTime(event.PreviousInclusionTime), epoch.IndexFromTime(event.InclusionTime)); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to update transaction inclusion time %s in epoch: %w", event.TransactionID, err))
		}
	}))
	// TODO: add transaction orphaned event

	e.Ledger.ConflictDAG.Events.ConflictCreated.Hook(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		e.NotarizationManager.IncreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(event.ID).IssuingTime()))
	}))
	e.Ledger.ConflictDAG.Events.ConflictAccepted.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}))
	e.Ledger.ConflictDAG.Events.ConflictRejected.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}))

	// Epochs are committed whenever ATT advances, start committing only when bootstrapped.
	e.Clock.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *clock.TimeUpdate) {
		e.NotarizationManager.SetAcceptanceTime(event.NewTime)
	}))

	e.Events.NotarizationManager.LinkTo(e.NotarizationManager.Events)
	e.Events.EpochMutations.LinkTo(e.NotarizationManager.EpochMutations.Events)
}

func (e *Engine) initEvictionState() {
	e.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		block.ForEachParent(func(parent models.Parent) {
			// TODO: ONLY ADD STRONG PARENTS AFTER NOT DOWNLOADING PAST WEAK ARROWS
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
		if err = file.Close(); err != nil {
			panic(err)
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

func WithSybilProtectionProvider(provider func(*Engine) sybilprotection.SybilProtection) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSybilProtectionProvider = provider
	}
}

func WithThroughputQuotaProvider(provider func(*Engine) throughputquota.ThroughputQuota) options.Option[Engine] {
	return func(e *Engine) {
		e.optsThroughputQuotaProvider = provider
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

func panicOnEmptyProvider[T any](engine *Engine) T {
	panic("provider not set")
}
