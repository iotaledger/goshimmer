package engine

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/storage"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/manatracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
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
	Events  *Events
	Storage *storage.Storage
	Ledger  *ledger.Ledger
	Inbox   *inbox.Inbox
	// SnapshotManager     *snapshot.Manager
	EvictionState       *eviction.State[models.BlockID]
	EntryPointsManager  *EntryPointsManager
	BlockRequester      *eventticker.EventTicker[models.BlockID]
	ManaTracker         *manatracker.ManaTracker
	NotarizationManager *notarization.Manager
	Tangle              *tangle.Tangle
	Consensus           *consensus.Consensus
	TSCManager          *tsc.Manager
	Clock               *clock.Clock
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	isBootstrapped      bool
	isBootstrappedMutex sync.Mutex

	optsBootstrappedThreshold      time.Duration
	optsEntryPointsDepth           int
	optsSnapshotFile               string
	optsSnapshotDepth              int
	optsLedgerOptions              []options.Option[ledger.Ledger]
	optsManaTrackerOptions         []options.Option[manatracker.ManaTracker]
	optsNotarizationManagerOptions []options.Option[notarization.Manager]
	optsTangleOptions              []options.Option[tangle.Tangle]
	optsConsensusOptions           []options.Option[consensus.Consensus]
	optsSybilProtectionOptions     []options.Option[sybilprotection.SybilProtection]
	optsTSCManagerOptions          []options.Option[tsc.Manager]
	optsDatabaseManagerOptions     []options.Option[database.Manager]
	optsBlockRequester             []options.Option[eventticker.EventTicker[models.BlockID]]
}

func New(storageInstance *storage.Storage, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(
		&Engine{
			Clock:              clock.New(),
			Events:             NewEvents(),
			ValidatorSet:       validator.NewSet(),
			EvictionState:      eviction.NewState[models.BlockID](),
			EntryPointsManager: NewEntryPointsManager(storageInstance),
			Storage:            storageInstance,

			optsEntryPointsDepth:      3,
			optsBootstrappedThreshold: 10 * time.Second,
			optsSnapshotFile:          "snapshot.bin",
			optsSnapshotDepth:         5,
		}, opts,
		(*Engine).initInbox,
		(*Engine).initLedger,
		(*Engine).initTangle,
		(*Engine).initConsensus,
		(*Engine).initClock,
		(*Engine).initTSCManager,
		(*Engine).initBlockStorage,
		(*Engine).initSybilProtection,
		(*Engine).initNotarizationManager,
		(*Engine).initManaTracker,
		(*Engine).initEvictionManager,
		(*Engine).initBlockRequester,
		(*Engine).initSolidEntryPointsManager,
	)
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

func (e *Engine) Evict(index epoch.Index) {
	solidEntryPoints := set.NewAdvancedSet[models.BlockID]()
	for i := index; i > index-epoch.Index(e.optsEntryPointsDepth); i-- {
		solidEntryPoints.AddAll(e.EntryPointsManager.LoadAll(i))
	}

	e.EvictionState.EvictUntil(index, solidEntryPoints)
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
	e.Storage.Shutdown()
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

func (e *Engine) initInbox() {
	e.Inbox = inbox.New()

	e.Events.Inbox = e.Inbox.Events
}

func (e *Engine) initLedger() {
	e.Ledger = ledger.New(e.Storage, e.optsLedgerOptions...)

	e.Events.Ledger = e.Ledger.Events
}

func (e *Engine) initTangle() {
	e.Tangle = tangle.New(e.Ledger, e.EvictionState, e.ValidatorSet, e.LastConfirmedEpoch, e.FirstUnacceptedMarker, e.optsTangleOptions...)

	e.Events.Inbox.BlockReceived.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := e.Tangle.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to attach block with %s (issuerID: %s): %w", block.ID(), block.IssuerID(), err))
		}
	}))

	e.Events.Tangle = e.Tangle.Events
}

func (e *Engine) initConsensus() {
	e.Consensus = consensus.New(e.Tangle, e.Storage.Permanent.Settings.LatestConfirmedEpoch(), func() int64 {
		weights := e.SybilProtection.Weights()
		var zeroID identity.ID
		var totalWeight int64
		for id, weight := range weights {
			if id != zeroID {
				totalWeight += weight
			}
		}
		return totalWeight
	}, e.optsConsensusOptions...)

	e.Events.Consensus = e.Consensus.Events

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

	e.Events.Clock = e.Clock.Events
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
	e.NotarizationManager = notarization.NewManager(e.Storage, e.SybilProtection)

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

	e.Events.NotarizationManager = e.NotarizationManager.Events
	e.Events.EpochMutations = e.NotarizationManager.EpochMutations.Events
}

func (e *Engine) initManaTracker() {
	e.ManaTracker = manatracker.New(e.Ledger, e.optsManaTrackerOptions...)
}

func (e *Engine) initSybilProtection() {
	e.SybilProtection = sybilprotection.New(e.ValidatorSet, e.Clock.RelativeAcceptedTime, e.optsSybilProtectionOptions...)

	e.Storage.Permanent.Events.ConsensusWeightsUpdated.Hook(event.NewClosure(e.SybilProtection.UpdateConsensusWeights))
	e.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(e.SybilProtection.TrackActiveValidators))
}

func (e *Engine) initEvictionManager() {
	e.NotarizationManager.Events.EpochCommitted.Attach(event.NewClosure(func(commitment *commitment.Commitment) {
		e.EvictionState.EvictUntil(commitment.Index(), e.EntryPointsManager.LoadAll(commitment.Index()))
	}))

	e.Events.EvictionManager = e.EvictionState.Events
}

func (e *Engine) initBlockRequester() {
	e.BlockRequester = eventticker.New(e.EvictionState, e.optsBlockRequester...)

	// We need to hook to make sure that the request is created before the block arrives to avoid a race condition
	// where we try to delete the request again before it is created. Thus, continuing to request forever.
	e.Events.Tangle.BlockDAG.BlockMissing.Hook(event.NewClosure(func(block *blockdag.Block) {
		// TODO: ONLY START REQUESTING WHEN NOT IN WARPSYNC RANGE (or just not attach outside)?
		e.BlockRequester.StartTicker(block.ID())
	}))
	e.Events.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.BlockRequester.StopTicker(block.ID())
	}))

	e.Events.BlockRequester = e.BlockRequester.Events
}

func (e *Engine) initSolidEntryPointsManager() {
	e.Events.Consensus.BlockGadget.BlockAccepted.Attach(event.NewClosure(func(block *blockgadget.Block) {
		e.EntryPointsManager.Insert(block.ID())
	}))

	e.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.EntryPointsManager.Remove(block.ID())
	}))
}

func (e *Engine) ProcessBlockFromPeer(block *models.Block, source identity.ID) {
	e.Inbox.ProcessReceivedBlock(block, source)
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

	if id.Index() > e.EvictionState.MaxEvictedEpoch() {
		return nil, false
	}

	block, err = e.Storage.Blocks.Load(id)
	exists = block != nil && err == nil

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

func WithSybilProtectionOptions(opts ...options.Option[sybilprotection.SybilProtection]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSybilProtectionOptions = opts
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

func WithSnapshotFile(snapshotFile string) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSnapshotFile = snapshotFile
	}
}

func WithDatabaseManagerOptions(opts ...options.Option[database.Manager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsDatabaseManagerOptions = opts
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
