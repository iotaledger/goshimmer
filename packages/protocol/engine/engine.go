package engine

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/activitylog"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/diskutil"
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
	logger                         *logger.Logger
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
}

func New(databaseVersion database.Version, chainDirectory string, logger *logger.Logger, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(
		&Engine{
			databaseVersion: databaseVersion,
			Clock:           clock.New(),
			Events:          NewEvents(),
			ValidatorSet:    validator.NewSet(),
			EvictionManager: eviction.NewManager[models.BlockID](),

			chainDirectory:            chainDirectory,
			logger:                    logger,
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

func (i *Engine) IsBootstrapped() (isBootstrapped bool) {
	// TODO: add bootstrapped flag from notarization
	return time.Since(i.Clock.RelativeAcceptedTime()) < i.optsBootstrappedThreshold
}

func (i *Engine) IsSynced() (isBootstrapped bool) {
	return i.IsBootstrapped() && time.Since(i.Clock.AcceptedTime()) < i.optsBootstrappedThreshold
}

func (i *Engine) initInbox() {
	i.Inbox = inbox.New()

	i.Events.Inbox = i.Inbox.Events
}

func (i *Engine) initDatabaseManager() {
	i.optsDatabaseManagerOptions = append(i.optsDatabaseManagerOptions, database.WithBaseDir(i.chainDirectory))

	i.DBManager = database.NewManager(i.databaseVersion, i.optsDatabaseManagerOptions...)
}

func (i *Engine) initLedger() {
	i.Ledger = ledger.New(append(i.optsLedgerOptions, ledger.WithStore(i.DBManager.PermanentStorage()))...)

	i.Events.Ledger = i.Ledger.Events
}

func (i *Engine) initTangle() {
	i.Tangle = tangle.New(i.Ledger, i.EvictionManager, i.ValidatorSet, i.optsTangleOptions...)

	i.Events.Inbox.BlockReceived.Attach(event.NewClosure(func(block *models.Block) {
		if _, _, err := i.Tangle.Attach(block); err != nil {
			i.Events.Error.Trigger(errors.Errorf("failed to attach block with %s: %w", block.ID(), err))
		}
	}))

	i.Events.Tangle = i.Tangle.Events
}

func (i *Engine) initConsensus() {
	i.Consensus = consensus.New(i.Tangle, i.optsConsensusOptions...)

	i.Events.Consensus = i.Consensus.Events
}

func (i *Engine) initClock() {
	i.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		i.Clock.SetAcceptedTime(block.IssuingTime())
	}))

	i.Events.Clock = i.Clock.Events
}

func (i *Engine) initTSCManager() {
	i.TSCManager = tsc.New(i.Consensus.IsBlockAccepted, i.Tangle, i.optsTSCManagerOptions...)

	i.Events.Tangle.Booker.BlockBooked.Attach(event.NewClosure(i.TSCManager.AddBlock))

	i.Events.Clock.AcceptanceTimeUpdated.Attach(event.NewClosure(func(e *clock.TimeUpdate) {
		i.TSCManager.HandleTimeUpdate(e.NewTime)
	}))
}

func (i *Engine) initBlockStorage() {
	i.BlockStorage = database.NewPersistentEpochStorage[models.BlockID, models.Block](i.DBManager, kvstore.Realm{0x09})

	i.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		i.BlockStorage.Set(block.ID(), block.ModelsBlock)
	}))

	i.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		i.BlockStorage.Delete(block.ID())
	}))
}

func (i *Engine) initNotarizationManager() {
	i.NotarizationManager = notarization.NewManager(
		i.Clock,
		i.Tangle,
		i.Ledger,
		i.Consensus,
		notarization.NewEpochCommitmentFactory(i.DBManager.PermanentStorage(), i.optsSnapshotDepth),
		append(i.optsNotarizationManagerOptions, notarization.ManaEpochDelay(mana.EpochDelay))...,
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

	i.Events.NotarizationManager = i.NotarizationManager.Events
}

func (i *Engine) initManaTracker() {
	i.ManaTracker = mana.NewTracker(i.Ledger, i.optsManaTrackerOptions...)

	i.NotarizationManager.Events.ManaVectorUpdate.Attach(i.ManaTracker.OnManaVectorToUpdateClosure)
}

func (i *Engine) initSnapshotManager() {
	i.SnapshotManager = snapshot.NewManager(i.NotarizationManager, 5)

	i.Events.Tangle.BlockDAG.BlockOrphaned.Attach(event.NewClosure(func(block *blockdag.Block) {
		i.SnapshotManager.RemoveSolidEntryPoint(block.ModelsBlock)
	}))

	i.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		block.ForEachParentByType(models.StrongParentType, func(parent models.BlockID) bool {
			if parent.EpochIndex < block.ID().EpochIndex {
				i.SnapshotManager.InsertSolidEntryPoint(parent)
			}

			return true
		})
	}))

	i.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(e *notarization.EpochCommittableEvent) {
		i.SnapshotManager.AdvanceSolidEntryPoints(e.EI)
	}))
}

func (i *Engine) initSybilProtection() {
	i.SybilProtection = sybilprotection.New(i.ValidatorSet, i.Clock.RelativeAcceptedTime, i.ManaTracker.GetConsensusMana)

	i.Events.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(i.SybilProtection.TrackActiveValidators))
}

func (i *Engine) initEvictionManager() {
	i.NotarizationManager.Events.EpochCommittable.Attach(event.NewClosure(func(event *notarization.EpochCommittableEvent) {
		i.EvictionManager.EvictUntil(event.EI, i.SnapshotManager.SolidEntryPoints(event.EI))
	}))

	i.Events.EvictionManager = i.EvictionManager.Events
}

func (i *Engine) Run() {
	i.loadSnapshot()
}

func (i *Engine) loadSnapshot() {
	if err := snapshot.LoadSnapshot(
		diskutil.New(i.chainDirectory).Path("snapshot.bin"),
		func(header *ledger.SnapshotHeader) {
			i.GenesisCommitment = header.LatestECRecord

			i.NotarizationManager.LoadECandEIs(header)

			i.ManaTracker.LoadSnapshotHeader(header)
		},
		i.SnapshotManager.LoadSolidEntryPoints,
		func(outputsWithMetadata []*ledger.OutputWithMetadata) {
			i.NotarizationManager.LoadOutputsWithMetadata(outputsWithMetadata)

			i.Ledger.LoadOutputsWithMetadata(outputsWithMetadata)

			i.ManaTracker.LoadOutputsWithMetadata(outputsWithMetadata)
		},
		func(epochDiffs *ledger.EpochDiff) {
			i.NotarizationManager.LoadEpochDiff(epochDiffs)

			if err := i.Ledger.LoadEpochDiff(epochDiffs); err != nil {
				panic(err)
			}

			i.ManaTracker.LoadEpochDiff(epochDiffs)
		},
		func(activityLogs activitylog.SnapshotEpochActivity) {
			for epoch, activityLog := range activityLogs {
				for issuerID, _ := range activityLog.NodesLog() {
					i.SybilProtection.AddValidator(issuerID, epoch.EndTime())
				}
			}
		},
	); err != nil {
		panic(err)
	}

	i.Clock.SetAcceptedTime(i.GenesisCommitment.Index().EndTime())
	i.EvictionManager.EvictUntil(i.GenesisCommitment.Index(), i.SnapshotManager.SolidEntryPoints(i.GenesisCommitment.Index()))
}

func (i *Engine) ProcessBlockFromPeer(block *models.Block, source identity.ID) {
	i.Inbox.ProcessReceivedBlock(block, source)
}

func (i *Engine) Block(id models.BlockID) (block *models.Block, exists bool) {
	if cachedBlock, cachedBlockExists := i.Tangle.BlockDAG.Block(id); cachedBlockExists {
		return cachedBlock.ModelsBlock, !cachedBlock.IsMissing()
	}

	if id.Index() > i.EvictionManager.MaxEvictedEpoch() {
		return nil, false
	}

	return i.BlockStorage.Get(id)
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
