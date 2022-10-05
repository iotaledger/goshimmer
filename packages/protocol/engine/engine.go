package engine

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/eventticker"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
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
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Engine /////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events              *Events
	Ledger              *ledger.Ledger
	GenesisCommitment   *commitment.Commitment
	BlockStorage        *database.PersistentEpochStorage[models.BlockID, models.Block, *models.BlockID, *models.Block]
	Inbox               *inbox.Inbox
	SnapshotManager     *snapshot.Manager
	EvictionManager     *eviction.Manager[models.BlockID]
	BlockRequester      *eventticker.EventTicker[models.BlockID]
	ManaTracker         *mana.Tracker
	NotarizationManager *notarization.Manager
	Tangle              *tangle.Tangle
	Consensus           *consensus.Consensus
	TSCManager          *tsc.TSCManager
	Clock               *clock.Clock
	SybilProtection     *sybilprotection.SybilProtection
	ValidatorSet        *validator.Set

	databaseVersion                database.Version
	chainDirectory                 string
	DBManager                      *database.Manager
	optsBootstrappedThreshold      time.Duration
	optsSnapshotFile               string
	optsSnapshotDepth              int
	optsLedgerOptions              []options.Option[ledger.Ledger]
	optsManaTrackerOptions         []options.Option[mana.Tracker]
	optsNotarizationManagerOptions []options.Option[notarization.Manager]
	optsTangleOptions              []options.Option[tangle.Tangle]
	optsConsensusOptions           []options.Option[consensus.Consensus]
	optsTSCManagerOptions          []options.Option[tsc.TSCManager]
	optsDatabaseManagerOptions     []options.Option[database.Manager]
	optsBlockRequester             []options.Option[eventticker.EventTicker[models.BlockID]]
}

func New(databaseVersion database.Version, chainDirectory string, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(
		&Engine{
			databaseVersion: databaseVersion,
			Clock:           clock.New(),
			Events:          NewEvents(),
			ValidatorSet:    validator.NewSet(),
			EvictionManager: eviction.NewManager[models.BlockID](),

			chainDirectory:            chainDirectory,
			optsBootstrappedThreshold: 10 * time.Second,
			optsSnapshotFile:          "snapshot.bin",
			optsSnapshotDepth:         5,
		}, opts,
		(*Engine).initInbox,
		(*Engine).initDatabaseManager,
		(*Engine).initLedger,
		(*Engine).initTangle,
		(*Engine).initConsensus,
		(*Engine).initClock,
		(*Engine).initTSCManager,
		(*Engine).initBlockStorage,
		(*Engine).initNotarizationManager,
		(*Engine).initManaTracker,
		(*Engine).initSnapshotManager,
		(*Engine).initSybilProtection,
		(*Engine).initEvictionManager,
	)
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	// TODO: add bootstrapped flag from notarization
	return time.Since(e.Clock.RelativeAcceptedTime()) < e.optsBootstrappedThreshold
}

func (e *Engine) IsSynced() (isBootstrapped bool) {
	return e.IsBootstrapped() && time.Since(e.Clock.AcceptedTime()) < e.optsBootstrappedThreshold
}

func (e *Engine) initInbox() {
	e.Inbox = inbox.New()

	e.Events.Inbox = e.Inbox.Events
}

func (e *Engine) initDatabaseManager() {
	e.optsDatabaseManagerOptions = append(e.optsDatabaseManagerOptions, database.WithBaseDir(e.chainDirectory))

	e.DBManager = database.NewManager(e.databaseVersion, e.optsDatabaseManagerOptions...)
}

func (e *Engine) initLedger() {
	e.Ledger = ledger.New(append(e.optsLedgerOptions, ledger.WithStore(e.DBManager.PermanentStorage()))...)

	e.Events.Ledger = e.Ledger.Events
}

func (e *Engine) initTangle() {
	e.Tangle = tangle.New(e.Ledger, e.EvictionManager, e.ValidatorSet, e.optsTangleOptions...)

	e.Events.Inbox.BlockReceived.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := e.Tangle.Attach(block); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to attach block with %s: %w", block.ID(), err))
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
	e.BlockStorage = database.NewPersistentEpochStorage[models.BlockID, models.Block](e.DBManager, kvstore.Realm{0x09})

	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		e.BlockStorage.Set(block.ID(), block.ModelsBlock)
	}))

	e.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.BlockStorage.Delete(block.ID())
	}))
}

func (e *Engine) initNotarizationManager() {
	e.NotarizationManager = notarization.NewManager(
		e.Clock,
		e.Tangle,
		e.Ledger,
		e.Consensus,
		notarization.NewCommitmentFactory(e.DBManager.PermanentStorage(), e.optsSnapshotDepth),
		append(e.optsNotarizationManagerOptions, notarization.ManaEpochDelay(mana.EpochDelay))...,
	)

	// i.Tangle.Events.BlockDAG.BlockAttached.Attach(event.NewClosure(i.NotarizationManager.OnBlockAttached))
	// i.Consensus.Gadget.Events.BlockAccepted.Attach(onlyIfBootstrapped(i, i.NotarizationManager.OnBlockAccepted))
	// i.Tangle.Events.BlockDAG.BlockOrphaned.Attach(onlyIfBootstrapped(i, i.NotarizationManager.OnBlockOrphaned))
	// i.Ledger.Events.TransactionAccepted.Attach(onlyIfBootstrapped(i, i.NotarizationManager.OnTransactionAccepted))
	// i.Ledger.Events.TransactionInclusionUpdated.Attach(onlyIfBootstrapped(i, i.NotarizationManager.OnTransactionInclusionUpdated))
	// i.Ledger.ConflictDAG.Events.ConflictAccepted.Attach(onlyIfBootstrapped(i, func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
	// 	i.NotarizationManager.OnConflictAccepted(event.ID)
	// }))
	// i.Ledger.ConflictDAG.Events.ConflictCreated.Attach(onlyIfBootstrapped(i, func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
	// 	i.NotarizationManager.OnConflictCreated(event.ID)
	// }))
	// i.Ledger.ConflictDAG.Events.ConflictRejected.Attach(onlyIfBootstrapped(i, func(event *conflictdag.ConflictRejectedEvent[utxo.TransactionID]) {
	// 	i.NotarizationManager.OnConflictRejected(event.ID)
	// }))
	// i.Clock.Events.AcceptanceTimeUpdated.Attach(onlyIfBootstrapped(i, func(event *clock.TimeUpdate) {
	// 	i.NotarizationManager.OnAcceptanceTimeUpdated(event.NewTime)
	// }))

	e.Events.NotarizationManager = e.NotarizationManager.Events
}

func (e *Engine) initManaTracker() {
	e.ManaTracker = mana.NewTracker(e.Ledger, e.optsManaTrackerOptions...)

	e.NotarizationManager.Events.ManaVectorUpdate.Attach(e.ManaTracker.OnManaVectorToUpdateClosure)
}

func (e *Engine) initSnapshotManager() {
	e.SnapshotManager = snapshot.NewManager(e.NotarizationManager, 5)

	e.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.SnapshotManager.RemoveSolidEntryPoint(block.ModelsBlock)
	}))

	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		block.ForEachParentByType(models.StrongParentType, func(parent models.BlockID) bool {
			if parent.EpochIndex < block.ID().EpochIndex {
				e.SnapshotManager.InsertSolidEntryPoint(parent)
			}

			return true
		})
	}))

	e.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		e.SnapshotManager.AdvanceSolidEntryPoints(event.EI)
	}))
}

func (e *Engine) initSybilProtection() {
	e.SybilProtection = sybilprotection.New(e.ValidatorSet, e.Clock.RelativeAcceptedTime, e.ManaTracker.GetConsensusMana)

	e.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(e.SybilProtection.TrackActiveValidators))
}

func (e *Engine) initEvictionManager() {
	e.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		e.EvictionManager.EvictUntil(event.EI, e.SnapshotManager.SolidEntryPoints(event.EI))
	}))

	e.Events.EvictionManager = e.EvictionManager.Events
}

func (e *Engine) initBlockRequester() {
	e.BlockRequester = eventticker.New(e.EvictionManager, e.optsBlockRequester...)

	e.Events.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(block *blockdag.Block) {
		// TODO: ONLY START REQUESTING WHEN NOT IN WARPSYNC RANGE (or just not attach outside)?
		e.BlockRequester.StartTicker(block.ID())
	}))
	e.Events.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		e.BlockRequester.StopTicker(block.ID())
	}))

	e.Events.BlockRequester = e.BlockRequester.Events
}

func (e *Engine) Run() {
	e.loadSnapshot()
}

func (e *Engine) loadSnapshot() {
	if err := snapshot.LoadSnapshot(
		diskutil.New(e.chainDirectory).Path("snapshot.bin"),
		func(header *ledger.SnapshotHeader) {
			e.GenesisCommitment = header.LatestECRecord

			e.NotarizationManager.LoadECandEIs(header)

			e.ManaTracker.LoadSnapshotHeader(header)
		},
		e.SnapshotManager.LoadSolidEntryPoints,
		func(outputsWithMetadata []*ledger.OutputWithMetadata) {
			e.NotarizationManager.LoadOutputsWithMetadata(outputsWithMetadata)

			e.Ledger.LoadOutputsWithMetadata(outputsWithMetadata)

			e.ManaTracker.LoadOutputsWithMetadata(outputsWithMetadata)
		},
		func(epochDiffs *ledger.EpochDiff) {
			e.NotarizationManager.LoadEpochDiff(epochDiffs)

			if err := e.Ledger.LoadEpochDiff(epochDiffs); err != nil {
				panic(err)
			}

			e.ManaTracker.LoadEpochDiff(epochDiffs)
		},
		func(activityLogs activitylog.SnapshotEpochActivity) {
			for epoch, activityLog := range activityLogs {
				for issuerID, _ := range activityLog.NodesLog() {
					e.SybilProtection.AddValidator(issuerID, epoch.EndTime())
				}
			}
		},
	); err != nil {
		panic(err)
	}

	e.Clock.SetAcceptedTime(e.GenesisCommitment.Index().EndTime())
	e.EvictionManager.EvictUntil(e.GenesisCommitment.Index(), e.SnapshotManager.SolidEntryPoints(e.GenesisCommitment.Index()))
}

func (e *Engine) ProcessBlockFromPeer(block *models.Block, source identity.ID) {
	e.Inbox.ProcessReceivedBlock(block, source)
}

func (e *Engine) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := e.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.ModelsBlock, !cachedBlock.IsMissing()
	}

	if id.Index() > e.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return e.BlockStorage.Get(id)
}

func emptyActivityConsumer(logs activitylog.SnapshotEpochActivity) {}

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

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensusOptions = opts
	}
}

func WithTSCManagerOptions(opts ...options.Option[tsc.TSCManager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTSCManagerOptions = opts
	}
}

func WithSnapshotFile(snapshotFile string) options.Option[Engine] {
	return func(p *Engine) {
		p.optsSnapshotFile = snapshotFile
	}
}

func WithDatabaseManagerOptions(opts ...options.Option[database.Manager]) options.Option[Engine] {
	return func(i *Engine) {
		i.optsDatabaseManagerOptions = opts
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[Engine] {
	return func(i *Engine) {
		i.optsLedgerOptions = opts
	}
}

func WithNotarizationManagerOptions(opts ...options.Option[notarization.Manager]) options.Option[Engine] {
	return func(i *Engine) {
		i.optsNotarizationManagerOptions = opts
	}
}

func WithSnapshotDepth(depth int) options.Option[Engine] {
	return func(i *Engine) {
		i.optsSnapshotDepth = depth
	}
}

func WithRequesterOptions(opts ...options.Option[eventticker.EventTicker[models.BlockID]]) options.Option[Engine] {
	return func(s *Engine) {
		s.optsBlockRequester = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
