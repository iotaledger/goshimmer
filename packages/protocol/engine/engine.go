package engine

import (
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
	"github.com/iotaledger/goshimmer/packages/storage"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/inbox"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana"
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
	ManaTracker         *mana.Tracker
	NotarizationManager *notarization.Manager
	Tangle              *tangle.Tangle
	Consensus           *consensus.Consensus
	TSCManager          *tsc.Manager
	Clock               *clock.Clock
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	optsBootstrappedThreshold      time.Duration
	optsEntryPointsDepth           int
	optsSnapshotFile               string
	optsSnapshotDepth              int
	optsLedgerOptions              []options.Option[ledger.Ledger]
	optsManaTrackerOptions         []options.Option[mana.Tracker]
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
		(*Engine).initNotarizationManager,
		(*Engine).initManaTracker,
		(*Engine).initSybilProtection,
		(*Engine).initEvictionManager,
		(*Engine).initBlockRequester,
		(*Engine).initSolidEntryPointsManager,
	)
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	// TODO: add bootstrapped flag from notarization
	return time.Since(e.Clock.RelativeAcceptedTime()) < e.optsBootstrappedThreshold
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

func (e *Engine) initInbox() {
	e.Inbox = inbox.New()

	e.Events.Inbox = e.Inbox.Events
}

func (e *Engine) initLedger() {
	e.Ledger = ledger.New(e.Storage, e.optsLedgerOptions...)

	e.Events.Ledger = e.Ledger.Events
}

func (e *Engine) initTangle() {
	e.Tangle = tangle.New(e.Ledger, e.EvictionState, e.ValidatorSet, e.optsTangleOptions...)

	e.Events.Inbox.BlockReceived.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := e.Tangle.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to attach block with %s (issuerID: %s): %w", block.ID(), block.IssuerID(), err))
		}
	}))

	e.Events.Tangle = e.Tangle.Events
}

func (e *Engine) initConsensus() {
	e.Consensus = consensus.New(e.Tangle, e.optsConsensusOptions...)

	e.Events.Consensus = e.Consensus.Events
}

func (e *Engine) initClock() {
	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		e.Clock.SetAcceptedTime(block.IssuingTime())
	}))

	e.Events.Clock = e.Clock.Events
}

func (e *Engine) initTSCManager() {
	e.TSCManager = tsc.New(e.Consensus.IsBlockAccepted, e.Tangle, e.optsTSCManagerOptions...)

	e.Events.Tangle.Booker.BlockBooked.Attach(event.NewClosure(e.TSCManager.AddBlock))

	e.Events.Clock.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *clock.TimeUpdate) {
		e.TSCManager.HandleTimeUpdate(event.NewTime)
	}))
}

func (e *Engine) initBlockStorage() {
	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
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
	e.NotarizationManager = notarization.NewManager(e.Storage)

	e.Consensus.Gadget.Events.BlockAccepted.Attach(onlyIfBootstrapped(e, func(block *acceptance.Block) {
		if err := e.NotarizationManager.AddAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted block %s to epoch: %w", block.ID(), err))
		}
	}))
	e.Tangle.Events.BlockDAG.BlockOrphaned.Attach(onlyIfBootstrapped(e, func(block *blockdag.Block) {
		if err := e.NotarizationManager.RemoveAcceptedBlock(block.ModelsBlock); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to remove orphaned block %s from epoch: %w", block.ID(), err))
		}
	}))

	e.Ledger.Events.TransactionAccepted.Attach(onlyIfBootstrapped(e, func(txMeta *ledger.TransactionMetadata) {
		if err := e.NotarizationManager.AddAcceptedTransaction(txMeta); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to add accepted transaction %s to epoch: %w", txMeta.ID(), err))
		}
	}))
	e.Ledger.Events.TransactionInclusionUpdated.Attach(onlyIfBootstrapped(e, func(event *ledger.TransactionInclusionUpdatedEvent) {
		if err := e.NotarizationManager.UpdateTransactionInclusion(event.TransactionID, epoch.IndexFromTime(event.PreviousInclusionTime), epoch.IndexFromTime(event.InclusionTime)); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to update transaction inclusion time %s in epoch: %w", event.TransactionID, err))
		}
	}))
	// TODO: add transaction orphaned event

	e.Ledger.ConflictDAG.Events.ConflictCreated.Hook(onlyIfBootstrapped(e, func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		e.NotarizationManager.IncreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(event.ID).IssuingTime()))
	}))
	e.Ledger.ConflictDAG.Events.ConflictAccepted.Hook(onlyIfBootstrapped(e, func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}))
	e.Ledger.ConflictDAG.Events.ConflictRejected.Hook(onlyIfBootstrapped(e, func(conflictID utxo.TransactionID) {
		e.NotarizationManager.DecreaseConflictsCounter(epoch.IndexFromTime(e.Tangle.GetEarliestAttachment(conflictID).IssuingTime()))
	}))

	e.Clock.Events.AcceptanceTimeUpdated.Attach(onlyIfBootstrapped(e, func(event *clock.TimeUpdate) {
		e.NotarizationManager.SetAcceptanceTime(event.NewTime)
	}))

	e.Events.NotarizationManager = e.NotarizationManager.Events
}

func (e *Engine) initManaTracker() {
	e.ManaTracker = mana.NewTracker(e.Ledger, e.Storage, e.optsManaTrackerOptions...)

	e.Ledger.Events.TransactionAccepted.Attach(event.NewClosure(e.ManaTracker.UpdateMana))
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
	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
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

func onlyIfBootstrapped[E any](engine *Engine, handler func(event E)) *event.Closure[E] {
	return event.NewClosure(func(event E) {
		if !engine.IsBootstrapped() {
			return
		}
		handler(event)
	})
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
