package engine

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/eventticker"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Engine /////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events              *Events
	Storage             *storage.Storage
	SybilProtection     sybilprotection.SybilProtection
	ThroughputQuota     throughputquota.ThroughputQuota
	Ledger              ledger.Ledger
	Filter              *filter.Filter
	EvictionState       *eviction.State
	BlockRequester      *eventticker.EventTicker[models.BlockID]
	NotarizationManager *notarization.Manager
	Tangle              *tangle.Tangle
	Consensus           *consensus.Consensus
	TSCManager          *tsc.Manager
	Clock               clock.Clock

	Workers *workerpool.Group

	isBootstrapped      bool
	isBootstrappedMutex sync.Mutex

	optsBootstrappedThreshold      time.Duration
	optsEntryPointsDepth           int
	optsSnapshotDepth              int
	optsNotarizationManagerOptions []options.Option[notarization.Manager]
	optsTangleOptions              []options.Option[tangle.Tangle]
	optsConsensusOptions           []options.Option[consensus.Consensus]
	optsTSCManagerOptions          []options.Option[tsc.Manager]
	optsBlockRequester             []options.Option[eventticker.EventTicker[models.BlockID]]
	optsFilter                     []options.Option[filter.Filter]

	module.Module
}

func New(
	workers *workerpool.Group,
	storageInstance *storage.Storage,
	clockProvider module.Provider[*Engine, clock.Clock],
	ledger module.Provider[*Engine, ledger.Ledger],
	sybilProtection module.Provider[*Engine, sybilprotection.SybilProtection],
	throughputQuota module.Provider[*Engine, throughputquota.ThroughputQuota],
	opts ...options.Option[Engine],
) (engine *Engine) {
	return options.Apply(
		&Engine{
			Events:        NewEvents(),
			Storage:       storageInstance,
			EvictionState: eviction.NewState(storageInstance),
			Workers:       workers,

			optsBootstrappedThreshold: 10 * time.Second,
			optsSnapshotDepth:         5,
		}, opts, func(e *Engine) {
			e.Ledger = ledger(e)
			e.Clock = clockProvider(e)
			e.SybilProtection = sybilProtection(e)
			e.ThroughputQuota = throughputQuota(e)
			e.NotarizationManager = notarization.NewManager(e.Storage, e.Ledger, e.SybilProtection.Weights(), e.optsNotarizationManagerOptions...)
			e.Tangle = tangle.New(e.Workers.CreateGroup("Tangle"), e.Ledger.MemPool(), e.EvictionState, e.SlotTimeProvider, e.SybilProtection.Validators(), e.LastConfirmedSlot, e.FirstUnacceptedMarker, e.Storage.Commitments.Load, e.optsTangleOptions...)
			e.Consensus = consensus.New(e.Workers.CreateGroup("Consensus"), e.Tangle, e.EvictionState, e.Storage.Permanent.Settings.LatestConfirmedSlot(), func() (totalWeight int64) {
				if zeroIdentityWeight, exists := e.SybilProtection.Weights().Get(identity.ID{}); exists {
					totalWeight -= zeroIdentityWeight.Value
				}

				return totalWeight + e.SybilProtection.Weights().TotalWeight()
			}, e.optsConsensusOptions...)
			e.TSCManager = tsc.New(e.Consensus.BlockGadget.IsBlockAccepted, e.Tangle, e.optsTSCManagerOptions...)
			e.Filter = filter.New(e.optsFilter...)
			e.BlockRequester = eventticker.New(e.optsBlockRequester...)

			e.HookInitialized(lo.Batch(
				e.Storage.Settings.TriggerInitialized,
				e.Storage.Commitments.TriggerInitialized,
				e.NotarizationManager.TriggerInitialized,
				func() {
					// TODO: hack until consensus is made an engine module
					e.Consensus.SlotGadget.SetLastConfirmedSlot(e.Storage.Permanent.Settings.LatestConfirmedSlot())
				},
			))
		},
		(*Engine).setupTangle,
		(*Engine).setupConsensus,
		(*Engine).setupTSCManager,
		(*Engine).setupBlockStorage,
		(*Engine).setupNotarizationManager,
		(*Engine).setupFilter,
		(*Engine).setupEvictionState,
		(*Engine).setupBlockRequester,
		(*Engine).TriggerConstructed,
	)
}

func (e *Engine) Shutdown() {
	if !e.WasStopped() {
		e.TriggerStopped()

		e.BlockRequester.Shutdown()
		e.Workers.Shutdown()
		e.Storage.Shutdown()
	}
}

func (e *Engine) ProcessBlockFromPeer(block *models.Block, source identity.ID) {
	e.Filter.ProcessReceivedBlock(block, source)
	e.Events.BlockProcessed.Trigger(block.ID())
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
	return e.Consensus.BlockGadget.FirstUnacceptedIndex(sequenceID)
}

func (e *Engine) LastConfirmedSlot() slot.Index {
	return e.Consensus.SlotGadget.LastConfirmedSlot()
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	e.isBootstrappedMutex.Lock()
	defer e.isBootstrappedMutex.Unlock()

	if e.isBootstrapped {
		return true
	}

	if isBootstrapped = time.Since(e.Clock.Accepted().RelativeTime()) < e.optsBootstrappedThreshold && e.NotarizationManager.IsFullyCommitted(); isBootstrapped {
		e.isBootstrapped = true
	}

	return isBootstrapped
}

func (e *Engine) IsSynced() (isBootstrapped bool) {
	return e.IsBootstrapped() && time.Since(e.Clock.Accepted().Time()) < e.optsBootstrappedThreshold
}

func (e *Engine) SlotTimeProvider() *slot.TimeProvider {
	return e.Storage.Settings.SlotTimeProvider()
}

func (e *Engine) Initialize(snapshot ...string) (err error) {
	if !e.Storage.Settings.SnapshotImported() {
		if len(snapshot) == 0 || snapshot[0] == "" {
			panic("no snapshot path specified")
		}
		if err = e.readSnapshot(snapshot[0]); err != nil {
			return errors.Wrapf(err, "failed to read snapshot from file '%s'", snapshot)
		}
	}

	e.TriggerInitialized()

	return
}

func (e *Engine) WriteSnapshot(filePath string, targetSlot ...slot.Index) (err error) {
	if len(targetSlot) == 0 {
		targetSlot = append(targetSlot, e.Storage.Settings.LatestCommitment().Index())
	}

	if fileHandle, err := os.Create(filePath); err != nil {
		return errors.Wrap(err, "failed to create snapshot file")
	} else if err = e.Export(fileHandle, targetSlot[0]); err != nil {
		return errors.Wrap(err, "failed to write snapshot")
	} else if err = fileHandle.Close(); err != nil {
		return errors.Wrap(err, "failed to close snapshot file")
	}

	return
}

func (e *Engine) Import(reader io.ReadSeeker) (err error) {
	if err = e.Storage.Settings.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import settings")
	} else if err = e.Storage.Commitments.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import commitments")
	} else if err = e.Storage.Settings.SetChainID(e.Storage.Settings.LatestCommitment().ID()); err != nil {
		return errors.Wrap(err, "failed to set chainID")
	} else if err = e.Ledger.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import ledger")
	} else if err = e.EvictionState.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import eviction state")
	} else if err = e.NotarizationManager.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import notarization state")
	}

	return
}

func (e *Engine) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	if err = e.Storage.Settings.Export(writer); err != nil {
		return errors.Wrap(err, "failed to export settings")
	} else if err = e.Storage.Commitments.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export commitments")
	} else if err = e.Ledger.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export ledger")
	} else if err = e.EvictionState.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export eviction state")
	} else if err = e.NotarizationManager.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export notarization state")
	}

	return
}

// RemoveFromFilesystem removes the directory of the engine from the filesystem.
func (e *Engine) RemoveFromFilesystem() error {
	return os.RemoveAll(e.Storage.Directory)
}

func (e *Engine) Name() string {
	return filepath.Base(e.Storage.Directory)
}

func (e *Engine) setupFilter() {
	e.Events.Filter.BlockFiltered.Hook(func(filteredEvent *filter.BlockFilteredEvent) {
		e.Events.Error.Trigger(errors.Wrapf(filteredEvent.Reason, "block (%s) filtered", filteredEvent.Block.ID()))
	}, event.WithWorkerPool(e.Workers.CreatePool("Filter", 2)))

	e.Events.Filter.LinkTo(e.Filter.Events)
}

func (e *Engine) setupTangle() {
	e.Events.Filter.BlockAllowed.Hook(func(block *models.Block) {
		if _, _, err := e.Tangle.BlockDAG.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to attach block with %s (issuerID: %s)", block.ID(), block.IssuerID()))
		}
	}, event.WithWorkerPool(e.Workers.CreatePool("Tangle.Attach", 2)))

	e.Events.NotarizationManager.SlotCommitted.Hook(func(evt *notarization.SlotCommittedDetails) {
		e.Tangle.BlockDAG.PromoteFutureBlocksUntil(evt.Commitment.Index())
	}, event.WithWorkerPool(e.Workers.CreatePool("Tangle.PromoteFutureBlocksUntil", 1)))

	e.Events.Tangle.LinkTo(e.Tangle.Events)
}

func (e *Engine) setupConsensus() {
	e.Events.Consensus.LinkTo(e.Consensus.Events)

	e.Events.Consensus.BlockGadget.Error.Hook(e.Events.Error.Trigger)
	e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
		err := e.Storage.Permanent.Settings.SetLatestConfirmedSlot(index)
		if err != nil {
			panic(err)
		}

		e.Tangle.Booker.VirtualVoting.EvictSlotTracker(index)
	}, event.WithWorkerPool(e.Workers.CreatePool("Consensus", 1))) // Using just 1 worker to avoid contention
}

func (e *Engine) setupTSCManager() {

	// wp := e.Workers.CreatePool("TSCManager", 1) // Using just 1 worker to avoid contention

	// TODO: enable TSC again
	// e.Events.Tangle.Booker.BlockBooked.Hook(e.TSCManager.AddBlock, event.WithWorkerPool(wp))
	// e.Events.Clock.AcceptanceTimeUpdated.Hook(func(event *clock.TimeUpdateEvent) {
	//	e.TSCManager.HandleTimeUpdate(event.NewTime)
	// }, event.WithWorkerPool(wp))
}

func (e *Engine) setupBlockStorage() {
	wp := e.Workers.CreatePool("BlockStorage", 1) // Using just 1 worker to avoid contention

	e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		if err := e.Storage.Blocks.Store(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to store block with %s", block.ID()))
		}
	}, event.WithWorkerPool(wp))
	e.Events.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
		if err := e.Storage.Blocks.Delete(block.ID()); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to delete block with %s", block.ID()))
		}
	}, event.WithWorkerPool(wp))
}

func (e *Engine) setupNotarizationManager() {
	e.Events.NotarizationManager.LinkTo(e.NotarizationManager.Events)
	e.Events.SlotMutations.LinkTo(e.NotarizationManager.SlotMutations.Events)

	wpBlocks := e.Workers.CreatePool("NotarizationManager.Blocks", 1)           // Using just 1 worker to avoid contention
	wpCommitments := e.Workers.CreatePool("NotarizationManager.Commitments", 1) // Using just 1 worker to avoid contention

	// SlotMutations must be hooked because inclusion might be added before transaction are added.
	e.Events.Ledger.MemPool.TransactionAccepted.Hook(func(event *mempool.TransactionEvent) {
		if err := e.NotarizationManager.SlotMutations.AddAcceptedTransaction(event.Metadata); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to add accepted transaction %s to slot", event.Metadata.ID()))
		}
	})
	e.Events.Ledger.MemPool.TransactionInclusionUpdated.Hook(func(event *mempool.TransactionInclusionUpdatedEvent) {
		if err := e.NotarizationManager.SlotMutations.UpdateTransactionInclusion(event.TransactionID, event.PreviousInclusionSlot, event.InclusionSlot); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to update transaction inclusion time %s in slot", event.TransactionID))
		}
	})

	e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		if err := e.NotarizationManager.NotarizeAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to add accepted block %s to slot", block.ID()))
		}
	}, event.WithWorkerPool(wpBlocks))
	e.Events.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
		if err := e.NotarizationManager.NotarizeOrphanedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Wrapf(err, "failed to remove orphaned block %s from slot", block.ID()))
		}
	}, event.WithWorkerPool(wpBlocks))

	// Slots are committed whenever ATT advances, start committing only when bootstrapped.
	e.Events.Clock.AcceptedTimeUpdated.Hook(e.NotarizationManager.SetAcceptanceTime, event.WithWorkerPool(wpCommitments))
}

func (e *Engine) setupEvictionState() {
	e.Ledger.HookInitialized(func() {
		e.EvictionState.EvictUntil(e.Storage.Settings.LatestCommitment().Index())
	})

	e.Events.EvictionState.LinkTo(e.EvictionState.Events)
	wp := e.Workers.CreatePool("EvictionState", 1) // Using just 1 worker to avoid contention

	e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
		block.ForEachParent(func(parent models.Parent) {
			// TODO: ONLY ADD STRONG PARENTS AFTER NOT DOWNLOADING PAST WEAK ARROWS
			// TODO: is this correct? could this lock acceptance in some extreme corner case? something like this happened, that confirmation is correctly advancing per block, but acceptance does not. I think it might have something to do with root blocks
			if parent.ID.Index() < block.ID().Index() {
				e.EvictionState.AddRootBlock(parent.ID)
			}
		})
	}, event.WithWorkerPool(wp))
	e.Events.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
		e.EvictionState.RemoveRootBlock(block.ID())
	}, event.WithWorkerPool(wp))
	e.Events.NotarizationManager.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
		e.EvictionState.EvictUntil(details.Commitment.Index())
	}, event.WithWorkerPool(wp))
}

func (e *Engine) setupBlockRequester() {
	e.Events.BlockRequester.LinkTo(e.BlockRequester.Events)

	e.Events.EvictionState.SlotEvicted.Hook(e.BlockRequester.EvictUntil)

	// We need to hook to make sure that the request is created before the block arrives to avoid a race condition
	// where we try to delete the request again before it is created. Thus, continuing to request forever.
	e.Events.Tangle.BlockDAG.BlockMissing.Hook(func(block *blockdag.Block) {
		// TODO: ONLY START REQUESTING WHEN NOT IN WARPSYNC RANGE (or just not attach outside)?
		e.BlockRequester.StartTicker(block.ID())
	})
	e.Events.Tangle.BlockDAG.MissingBlockAttached.Hook(func(block *blockdag.Block) {
		e.BlockRequester.StopTicker(block.ID())
	}, event.WithWorkerPool(e.Workers.CreatePool("BlockRequester", 1))) // Using just 1 worker to avoid contention
}

func (e *Engine) readSnapshot(filePath string) (err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to open snapshot file")
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			panic(closeErr)
		}
	}()

	if err = e.Import(file); err != nil {
		return errors.Wrap(err, "failed to import snapshot")
	} else if err = e.Storage.Settings.SetSnapshotImported(true); err != nil {
		return errors.Wrap(err, "failed to set snapshot imported flag")
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
		e.optsTangleOptions = append(e.optsTangleOptions, opts...)
	}
}

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensusOptions = append(e.optsConsensusOptions, opts...)
	}
}

func WithEntryPointsDepth(entryPointsDepth int) options.Option[Engine] {
	return func(engine *Engine) {
		engine.optsEntryPointsDepth = entryPointsDepth
	}
}

func WithTSCManagerOptions(opts ...options.Option[tsc.Manager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTSCManagerOptions = append(e.optsTSCManagerOptions, opts...)
	}
}

func WithFilterOptions(opts ...options.Option[filter.Filter]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsFilter = append(e.optsFilter, opts...)
	}
}

func WithNotarizationManagerOptions(opts ...options.Option[notarization.Manager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsNotarizationManagerOptions = append(e.optsNotarizationManagerOptions, opts...)
	}
}

func WithSnapshotDepth(depth int) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSnapshotDepth = depth
	}
}

func WithRequesterOptions(opts ...options.Option[eventticker.EventTicker[models.BlockID]]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsBlockRequester = append(e.optsBlockRequester, opts...)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
